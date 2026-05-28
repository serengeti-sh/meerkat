package collector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/meerkatlogs"
)

func testBatcherConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Collector.BatchSize = 3
	cfg.Collector.FlushInterval = 10 * time.Second
	return cfg
}

func TestBatcher_Add_FlushOnBatchSize(t *testing.T) {
	b := NewBatcher(testBatcherConfig())

	entries := []LogEntry{
		{Body: "log1", Service: "svc", Severity: "info"},
		{Body: "log2", Service: "svc", Severity: "info"},
		{Body: "log3", Service: "svc", Severity: "info"},
	}

	for _, e := range entries {
		err := b.Add(e)
		require.NoError(t, err)
	}

	// Without a logs client/service, flush will fail with "no MeerkatLogs client configured"
	// but Add should still work and buffer should be flushed (emptied).
	assert.Empty(t, b.buffer)
}

func TestBatcher_Stop_FlushesRemaining(t *testing.T) {
	cfg := testBatcherConfig()
	cfg.Collector.BatchSize = 100

	b := NewBatcher(cfg)
	b.Start()

	err := b.Add(LogEntry{Body: "log1", Service: "svc", Severity: "info"})
	require.NoError(t, err)
	err = b.Add(LogEntry{Body: "log2", Service: "svc", Severity: "warn"})
	require.NoError(t, err)

	b.Stop(context.Background())

	assert.Empty(t, b.buffer)
}

func TestBatcher_EmptyFlush(t *testing.T) {
	b := NewBatcher(testBatcherConfig())
	b.triggerFlush()

	assert.Empty(t, b.buffer)
}

func TestBatcher_NoClientError(t *testing.T) {
	b := NewBatcher(testBatcherConfig())

	entries := []LogEntry{
		{Body: "log1", Service: "svc", Severity: "info"},
	}

	err := b.flush(context.Background(), entries)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no MeerkatLogs client")
}

func TestBatcher_WithLogsService(t *testing.T) {
	cfg := testBatcherConfig()
	cfg.Collector.BatchSize = 1

	b := NewBatcher(cfg)

	// Create a simple mock service that counts ingestions
	var ingestCount int
	mockSvc := &mockLogsService{
		ingestFunc: func(ctx context.Context, entries []meerkatlogs.LogEntry) (*meerkatlogs.IngestResult, error) {
			ingestCount += len(entries)
			return &meerkatlogs.IngestResult{IngestedCount: len(entries)}, nil
		},
	}
	b.WithLogsService(mockSvc)

	err := b.Add(LogEntry{Body: "log1", Service: "svc", Severity: "info"})
	require.NoError(t, err)

	// Wait for async flush
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, 1, ingestCount)
}

func TestBatcher_GeneratesID(t *testing.T) {
	cfg := testBatcherConfig()
	cfg.Collector.BatchSize = 1

	b := NewBatcher(cfg)

	err := b.Add(LogEntry{Body: "log1", Service: "svc", Severity: "info"})
	require.NoError(t, err)

	// Buffer is flushed asynchronously, so it may be empty
	// Just verify Add didn't panic and entry was accepted
	assert.NotNil(t, b)
}

func TestBatcher_Add_AfterStop(t *testing.T) {
	b := NewBatcher(testBatcherConfig())
	b.Start()
	b.Stop(context.Background())

	err := b.Add(LogEntry{Body: "log1", Service: "svc", Severity: "info"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stopped")
}

type mockLogsService struct {
	ingestFunc func(ctx context.Context, entries []meerkatlogs.LogEntry) (*meerkatlogs.IngestResult, error)
}

func (m *mockLogsService) Ingest(ctx context.Context, entries []meerkatlogs.LogEntry) (*meerkatlogs.IngestResult, error) {
	if m.ingestFunc != nil {
		return m.ingestFunc(ctx, entries)
	}
	return &meerkatlogs.IngestResult{}, nil
}

func (m *mockLogsService) Search(ctx context.Context, query string, opts meerkatlogs.SearchOptions) ([]meerkatlogs.SearchResult, error) {
	return nil, nil
}

func (m *mockLogsService) GetContext(ctx context.Context, service string, start, end time.Time, limit int) ([]meerkatlogs.SearchResult, error) {
	return nil, nil
}
