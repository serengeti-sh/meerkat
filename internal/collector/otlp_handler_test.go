package collector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1data "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
)

func TestOTLPHandler_Export(t *testing.T) {
	var collected []LogEntry
	batcher := &testBatcher{onAdd: func(e LogEntry) {
		collected = append(collected, e)
	}}

	handler := &otlpHandler{batcher: batcher}

	req := &logsv1.ExportLogsServiceRequest{
		ResourceLogs: []*logsv1data.ResourceLogs{
			{
				Resource: &resourcev1.Resource{
					Attributes: []*commonv1.KeyValue{
						{
							Key: "service.name",
							Value: &commonv1.AnyValue{
								Value: &commonv1.AnyValue_StringValue{StringValue: "my-service"},
							},
						},
					},
				},
				ScopeLogs: []*logsv1data.ScopeLogs{
					{
						LogRecords: []*logsv1data.LogRecord{
							{
								TimeUnixNano: 1700000000_000000000,
								SeverityText: "ERROR",
								Body: &commonv1.AnyValue{
									Value: &commonv1.AnyValue_StringValue{StringValue: "connection refused"},
								},
								Attributes: []*commonv1.KeyValue{
									{
										Key: "host",
										Value: &commonv1.AnyValue{
											Value: &commonv1.AnyValue_StringValue{StringValue: "server-1"},
										},
									},
								},
							},
							{
								TimeUnixNano: 1700000001_000000000,
								SeverityText: "INFO",
								Body: &commonv1.AnyValue{
									Value: &commonv1.AnyValue_StringValue{StringValue: "request completed"},
								},
							},
						},
					},
				},
			},
		},
	}

	resp, err := handler.Export(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	require.Len(t, collected, 2)

	assert.Equal(t, "my-service", collected[0].Service)
	assert.Equal(t, "ERROR", collected[0].Severity)
	assert.Equal(t, "connection refused", collected[0].Body)
	assert.Equal(t, "server-1", collected[0].Attributes["host"])

	assert.Equal(t, "INFO", collected[1].Severity)
	assert.Equal(t, "request completed", collected[1].Body)
}

func TestOTLPHandler_Export_EmptyRequest(t *testing.T) {
	var collected []LogEntry
	handler := &otlpHandler{batcher: &testBatcher{onAdd: func(e LogEntry) {
		collected = append(collected, e)
	}}}

	resp, err := handler.Export(context.Background(), &logsv1.ExportLogsServiceRequest{})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, collected)
}

func TestOTLPHandler_Export_NilResource(t *testing.T) {
	var collected []LogEntry
	handler := &otlpHandler{batcher: &testBatcher{onAdd: func(e LogEntry) {
		collected = append(collected, e)
	}}}

	req := &logsv1.ExportLogsServiceRequest{
		ResourceLogs: []*logsv1data.ResourceLogs{
			{
				ScopeLogs: []*logsv1data.ScopeLogs{
					{
						LogRecords: []*logsv1data.LogRecord{
							{
								TimeUnixNano: 1700000000_000000000,
								Body: &commonv1.AnyValue{
									Value: &commonv1.AnyValue_StringValue{StringValue: "test log"},
								},
							},
						},
					},
				},
			},
		},
	}

	resp, err := handler.Export(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	require.Len(t, collected, 1)
	assert.Equal(t, "", collected[0].Service)
}

func TestLogRecordToEntry_IntBody(t *testing.T) {
	lr := &logsv1data.LogRecord{
		TimeUnixNano: 1700000000_000000000,
		SeverityText: "WARN",
		Body: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_IntValue{IntValue: 42},
		},
	}

	entry := logRecordToEntry(lr, "svc")
	assert.Equal(t, "42", entry.Body)
}

func TestLogRecordToEntry_BoolBody(t *testing.T) {
	lr := &logsv1data.LogRecord{
		TimeUnixNano: 1700000000_000000000,
		Body: &commonv1.AnyValue{
			Value: &commonv1.AnyValue_BoolValue{BoolValue: true},
		},
	}

	entry := logRecordToEntry(lr, "svc")
	assert.Equal(t, "true", entry.Body)
}

func TestLogRecordToEntry_NilBody(t *testing.T) {
	lr := &logsv1data.LogRecord{
		TimeUnixNano: 1700000000_000000000,
	}

	entry := logRecordToEntry(lr, "svc")
	assert.Equal(t, "", entry.Body)
}

func TestExtractResourceAttributes_Nil(t *testing.T) {
	result := extractResourceAttributes(nil)
	assert.Nil(t, result)
}

type testBatcher struct {
	onAdd func(LogEntry)
}

func (b *testBatcher) Add(entry LogEntry) error {
	if b.onAdd != nil {
		b.onAdd(entry)
	}
	return nil
}
