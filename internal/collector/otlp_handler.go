package collector

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

// LogSink accepts log entries for batching.
type LogSink interface {
	Add(entry LogEntry) error
}

// otlpHandler implements the OTLP LogsServiceServer.
type otlpHandler struct {
	logsv1.UnimplementedLogsServiceServer
	batcher LogSink
}

func (h *otlpHandler) Export(ctx context.Context, req *logsv1.ExportLogsServiceRequest) (*logsv1.ExportLogsServiceResponse, error) {
	for _, rl := range req.ResourceLogs {
		resourceAttrs := extractResourceAttributes(rl.Resource)
		serviceName := resourceAttrs["service.name"]

		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				entry := logRecordToEntry(lr, serviceName)
				if err := h.batcher.Add(entry); err != nil {
					return nil, status.Errorf(codes.ResourceExhausted, "batcher rejected log: %v", err)
				}
			}
		}
	}

	return &logsv1.ExportLogsServiceResponse{}, nil
}

func logRecordToEntry(lr *logsv1data.LogRecord, serviceName string) LogEntry {
	entry := LogEntry{
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
