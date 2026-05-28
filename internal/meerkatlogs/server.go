package meerkatlogs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/serengeti-sh/meerkat/internal/meerkatlogspb"
)

// GRPCServer implements the meerkatlogspb.ServiceServer interface.
type GRPCServer struct {
	meerkatlogspb.UnimplementedServiceServer
	svc Service
}

var _ meerkatlogspb.ServiceServer = (*GRPCServer)(nil)

// NewGRPCServer creates a gRPC server for the MeerkatLogs service.
func NewGRPCServer(svc Service) (*GRPCServer, error) {
	if svc == nil {
		return nil, fmt.Errorf("rag: svc is required")
	}
	return &GRPCServer{svc: svc}, nil
}

func toProto(r SearchResult) *meerkatlogspb.SearchResult {
	return &meerkatlogspb.SearchResult{
		Id:        r.ID,
		Score:     r.Score,
		Body:      r.Body,
		Service:   r.Service,
		Severity:  r.Severity,
		Timestamp: timestamppb.New(r.Timestamp),
	}
}

// Ingest adds log entries to the vector store.
func (s *GRPCServer) Ingest(ctx context.Context, req *meerkatlogspb.IngestRequest) (*meerkatlogspb.IngestResponse, error) {
	entries := make([]LogEntry, len(req.Entries))
	for i, e := range req.Entries {
		entries[i] = LogEntry{
			ID:         e.Id,
			Timestamp:  e.Timestamp.AsTime(),
			Service:    e.Service,
			Severity:   e.Severity,
			Body:       e.Body,
			Attributes: e.Attributes,
		}
	}

	result, err := s.svc.Ingest(ctx, entries)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
	}

	return &meerkatlogspb.IngestResponse{
		IngestedCount:     int32(result.IngestedCount),
		DeduplicatedCount: int32(result.DeduplicatedCount),
	}, nil
}

// Search finds semantically similar log entries.
func (s *GRPCServer) Search(ctx context.Context, req *meerkatlogspb.SearchRequest) (*meerkatlogspb.SearchResponse, error) {
	opts := SearchOptions{
		Limit:     int(req.Limit),
		TimeRange: time.Duration(req.TimeRangeSeconds) * time.Second,
		Service:   req.Service,
		Severity:  req.Severity,
	}

	results, err := s.svc.Search(ctx, req.Query, opts)
	if err != nil {
		if errors.Is(err, ErrEmptyQuery) {
			return nil, status.Errorf(codes.InvalidArgument, "empty query")
		}
		if errors.Is(err, ErrNoResults) {
			return &meerkatlogspb.SearchResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	protoResults := make([]*meerkatlogspb.SearchResult, len(results))
	for i, r := range results {
		protoResults[i] = toProto(r)
	}

	return &meerkatlogspb.SearchResponse{Results: protoResults}, nil
}

// GetContext retrieves relevant log context for a given service and time range.
func (s *GRPCServer) GetContext(ctx context.Context, req *meerkatlogspb.GetContextRequest) (*meerkatlogspb.GetContextResponse, error) {
	results, err := s.svc.GetContext(
		ctx,
		req.Service,
		req.StartTime.AsTime(),
		req.EndTime.AsTime(),
		int(req.Limit),
	)
	if err != nil {
		if errors.Is(err, ErrInvalidTimeRange) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid time range")
		}
		return nil, status.Errorf(codes.Internal, "get context failed: %v", err)
	}

	protoResults := make([]*meerkatlogspb.SearchResult, len(results))
	for i, r := range results {
		protoResults[i] = toProto(r)
	}

	return &meerkatlogspb.GetContextResponse{Results: protoResults}, nil
}
