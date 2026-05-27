package collector

import (
	"fmt"
	"log"
	"net"

	"github.com/serengeti-sh/meerkat/internal/config"
	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
)

// GRPCServer wraps a gRPC server that receives OTLP log exports.
type GRPCServer struct {
	srv     *grpc.Server
	addr    string
	batcher LogSink
}

// NewGRPCServer creates a GRPCServer with the given configuration.
func NewGRPCServer(cfg *config.Config, batcher LogSink) *GRPCServer {
	return &GRPCServer{
		addr:    cfg.Collector.OTLPBindAddr,
		batcher: batcher,
	}
}

// Start begins listening for OTLP log exports.
func (s *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}

	s.srv = grpc.NewServer()
	logsv1.RegisterLogsServiceServer(s.srv, &otlpHandler{batcher: s.batcher})

	go func() {
		if err := s.srv.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the gRPC server.
func (s *GRPCServer) Stop() {
	if s.srv != nil {
		s.srv.GracefulStop()
	}
}
