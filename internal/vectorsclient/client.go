package vectorsclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/serengeti-sh/meerkat/internal/vectorspb"
)

// Client is a gRPC client for the Vectors service.
type Client interface {
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)
	GetContext(ctx context.Context, service string, start, end time.Time, limit int) ([]SearchResult, error)
	Close() error
}

type client struct {
	conn *grpc.ClientConn
	cli  vectorspb.ServiceClient
}

var _ Client = (*client)(nil)

// New creates a client connected to the Vectors gRPC server at addr.
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
		return nil, fmt.Errorf("connect to Vectors server: %w", err)
	}

	return &client{
		conn: conn,
		cli:  vectorspb.NewServiceClient(conn),
	}, nil
}

func (c *client) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	req := &vectorspb.SearchRequest{
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
	req := &vectorspb.GetContextRequest{
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

func fromProtoResults(results []*vectorspb.SearchResult) []SearchResult {
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
