package vectors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/serengeti-sh/meerkat/internal/vectorspb"
)

// GRPCServer implements the vectorspb.ServiceServer interface.
type GRPCServer struct {
	vectorspb.UnimplementedServiceServer
	svc Service
}

var _ vectorspb.ServiceServer = (*GRPCServer)(nil)

// NewGRPCServer creates a gRPC server for the Vectors service.
func NewGRPCServer(svc Service) (*GRPCServer, error) {
	if svc == nil {
		return nil, fmt.Errorf("vectors: svc is required")
	}
	return &GRPCServer{svc: svc}, nil
}

func toProto(r SearchResult) *vectorspb.SearchResult {
	return &vectorspb.SearchResult{
		Id:        r.ID,
		Score:     r.Score,
		Body:      r.Body,
		Service:   r.Service,
		Severity:  r.Severity,
		Timestamp: timestamppb.New(r.Timestamp),
	}
}

// Search finds semantically similar log entries.
func (s *GRPCServer) Search(ctx context.Context, req *vectorspb.SearchRequest) (*vectorspb.SearchResponse, error) {
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
			return &vectorspb.SearchResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	protoResults := make([]*vectorspb.SearchResult, len(results))
	for i, r := range results {
		protoResults[i] = toProto(r)
	}

	return &vectorspb.SearchResponse{Results: protoResults}, nil
}

// GetContext retrieves relevant log context for a given service and time range.
func (s *GRPCServer) GetContext(ctx context.Context, req *vectorspb.GetContextRequest) (*vectorspb.GetContextResponse, error) {
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

	protoResults := make([]*vectorspb.SearchResult, len(results))
	for i, r := range results {
		protoResults[i] = toProto(r)
	}

	return &vectorspb.GetContextResponse{Results: protoResults}, nil
}
