package ragclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/serengeti-sh/meerkat/internal/rag/ragpb"
)

// Client is a gRPC client for the RAG service.
type Client interface {
	Ingest(ctx context.Context, entries []LogEntry) (*IngestResult, error)
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
	GetContext(ctx context.Context, service string, start, end time.Time, limit int) ([]SearchResult, error)
	Close() error
}

type client struct {
	conn *grpc.ClientConn
	cli  ragpb.RAGServiceClient
}

var _ Client = (*client)(nil)

// New creates a Client connected to the RAG gRPC server at addr.
func New(addr string, opts ...Option) (Client, error) {
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
		return nil, fmt.Errorf("connect to rag server: %w", err)
	}

	return &client{
		conn: conn,
		cli:  ragpb.NewRAGServiceClient(conn),
	}, nil
}

func (c *client) Ingest(ctx context.Context, entries []LogEntry) (*IngestResult, error) {
	reqEntries := make([]*ragpb.LogEntry, len(entries))
	for i, e := range entries {
		reqEntries[i] = &ragpb.LogEntry{
			Id:         e.ID,
			Timestamp:  timestamppb.New(e.Timestamp),
			Service:    e.Service,
			Severity:   e.Severity,
			Body:       e.Body,
			Attributes: e.Attributes,
		}
	}

	resp, err := c.cli.Ingest(ctx, &ragpb.IngestRequest{Entries: reqEntries})
	if err != nil {
		return nil, fmt.Errorf("ingest: %w", err)
	}

	return &IngestResult{
		IngestedCount:     int(resp.IngestedCount),
		DeduplicatedCount: int(resp.DeduplicatedCount),
	}, nil
}

func (c *client) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	req := &ragpb.SearchRequest{
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
	req := &ragpb.GetContextRequest{
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

func fromProtoResults(results []*ragpb.SearchResult) []SearchResult {
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
