package vectors

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1data "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
)

// OTLPIngestor receives logs via OTLP/gRPC and forwards them to the vectors Service.
type OTLPIngestor struct {
	addr       string
	grpcServer *grpc.Server
	listener   net.Listener
}

// NewOTLPIngestor creates an OTLP ingestor that will listen on the given address.
func NewOTLPIngestor(addr string) *OTLPIngestor {
	return &OTLPIngestor{addr: addr}
}

// Name returns the ingestor identifier.
func (o *OTLPIngestor) Name() string {
	return "otlp"
}

// Start begins the OTLP gRPC server and registers the logs receiver.
func (o *OTLPIngestor) Start(ctx context.Context, svc Service) error {
	lis, err := net.Listen("tcp", o.addr)
	if err != nil {
		return fmt.Errorf("listen otlp: %w", err)
	}
	o.listener = lis

	o.grpcServer = grpc.NewServer()
	logsv1.RegisterLogsServiceServer(o.grpcServer, &otlpLogsServer{svc: svc})

	go func() {
		log.Printf("OTLP ingestor listening on %s", o.addr)
		if err := o.grpcServer.Serve(lis); err != nil {
			log.Printf("OTLP server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the OTLP gRPC server.
func (o *OTLPIngestor) Stop(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		o.grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		o.grpcServer.Stop()
	}
	return nil
}

// otlpLogsServer implements the OTLP LogsServiceServer.
type otlpLogsServer struct {
	logsv1.UnimplementedLogsServiceServer
	svc Service
}

// Export handles OTLP ExportLogsServiceRequest.
func (s *otlpLogsServer) Export(ctx context.Context, req *logsv1.ExportLogsServiceRequest) (*logsv1.ExportLogsServiceResponse, error) {
	var entries []Entry

	for _, rl := range req.ResourceLogs {
		resourceAttrs := extractResourceAttributes(rl.Resource)
		serviceName := resourceAttrs["service.name"]

		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				entries = append(entries, logRecordToEntry(lr, serviceName))
			}
		}
	}

	if len(entries) > 0 {
		if _, err := s.svc.Ingest(ctx, entries); err != nil {
			return nil, status.Errorf(codes.Internal, "ingest failed: %v", err)
		}
	}

	return &logsv1.ExportLogsServiceResponse{}, nil
}

func logRecordToEntry(lr *logsv1data.LogRecord, serviceName string) Entry {
	entry := Entry{
		Timestamp:  time.Unix(0, int64(lr.TimeUnixNano)),
		Severity:   lr.SeverityText,
		Service:    serviceName,
		Body:       "",
		Attributes: make(map[string]string),
	}

	if lr.Body != nil {
		entry.Body = anyValueToString(lr.Body)
	}

	for _, attr := range lr.Attributes {
		entry.Attributes[attr.Key] = anyValueToString(attr.Value)
	}

	return entry
}

func anyValueToString(v *commonv1.AnyValue) string {
	if v == nil {
		return ""
	}

	switch val := v.Value.(type) {
	case *commonv1.AnyValue_StringValue:
		return val.StringValue
	case *commonv1.AnyValue_IntValue:
		return strconv.FormatInt(val.IntValue, 10)
	case *commonv1.AnyValue_DoubleValue:
		return strconv.FormatFloat(val.DoubleValue, 'f', -1, 64)
	case *commonv1.AnyValue_BoolValue:
		return strconv.FormatBool(val.BoolValue)
	default:
		return ""
	}
}

func extractResourceAttributes(r *resourcev1.Resource) map[string]string {
	if r == nil {
		return nil
	}

	attrs := make(map[string]string)
	for _, attr := range r.Attributes {
		attrs[attr.Key] = anyValueToString(attr.Value)
	}
	return attrs
}
