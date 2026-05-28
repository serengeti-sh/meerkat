package logsclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/serengeti-sh/meerkat/internal/meerkatlogspb"
)

// Client is a gRPC client for the MeerkatLogs service.
type Client interface {
	Ingest(ctx context.Context, entries []LogEntry) (*IngestResult, error)
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
	GetContext(ctx context.Context, service string, start, end time.Time, limit int) ([]SearchResult, error)
	Close() error
}

type client struct {
	conn *grpc.ClientConn
	cli  meerkatlogspb.ServiceClient
}

var _ Client = (*client)(nil)

// New creates a client connected to the RAG gRPC server at addr.
func New(addr string, opts ...Option) (*client, error) {
	cfg := &config{
		dialOpts: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	conn, err := grpc.NewClient(addr, cfg.dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("connect to MeerkatLogs server: %w", err)
	}

	return &client{
		conn: conn,
		cli:  meerkatlogspb.NewServiceClient(conn),
	}, nil
}

func (c *client) Ingest(ctx context.Context, entries []LogEntry) (*IngestResult, error) {
	reqEntries := make([]*meerkatlogspb.LogEntry, len(entries))
	for i, e := range entries {
		reqEntries[i] = &meerkatlogspb.LogEntry{
			Id:         e.ID,
			Timestamp:  timestamppb.New(e.Timestamp),
			Service:    e.Service,
			Severity:   e.Severity,
			Body:       e.Body,
			Attributes: e.Attributes,
		}
	}

	resp, err := c.cli.Ingest(ctx, &meerkatlogspb.IngestRequest{Entries: reqEntries})
	if err != nil {
		return nil, fmt.Errorf("ingest: %w", err)
	}

	return &IngestResult{
		IngestedCount:     int(resp.IngestedCount),
		DeduplicatedCount: int(resp.DeduplicatedCount),
	}, nil
}

func (c *client) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	req := &meerkatlogspb.SearchRequest{
		Query:            query,
		Limit:            int32(opts.Limit),
		TimeRangeSeconds: int64(opts.TimeRange.Seconds()),
		Service:          opts.Service,
		Severity:         opts.Severity,
	}

	resp, err := c.cli.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	return fromProtoResults(resp.Results), nil
}

func (c *client) GetContext(ctx context.Context, service string, start, end time.Time, limit int) ([]SearchResult, error) {
	req := &meerkatlogspb.GetContextRequest{
		Service:   service,
		StartTime: timestamppb.New(start),
		EndTime:   timestamppb.New(end),
		Limit:     int32(limit),
	}

	resp, err := c.cli.GetContext(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get context: %w", err)
	}

	return fromProtoResults(resp.Results), nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func fromProtoResults(results []*meerkatlogspb.SearchResult) []SearchResult {
	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{
			ID:        r.Id,
			Score:     r.Score,
			Body:      r.Body,
			Service:   r.Service,
			Severity:  r.Severity,
			Timestamp: r.Timestamp.AsTime(),
		}
	}
	return out
}
