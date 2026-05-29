package vectors

import (
	"context"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1data "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
)

// OTLPServer implements the OTLP LogsServiceServer.
// It receives OTLP logs and forwards them to the vectors Service.
type OTLPServer struct {
	logsv1.UnimplementedLogsServiceServer
	svc Service
}

// NewOTLPServer creates an OTLP server that forwards to the given service.
func NewOTLPServer(svc Service) *OTLPServer {
	return &OTLPServer{svc: svc}
}

// Export handles OTLP ExportLogsServiceRequest.
func (s *OTLPServer) Export(ctx context.Context, req *logsv1.ExportLogsServiceRequest) (*logsv1.ExportLogsServiceResponse, error) {
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
