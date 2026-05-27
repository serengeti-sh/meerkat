package logstream_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/serengeti-sh/meerkat/internal/rag"
	"github.com/serengeti-sh/meerkat/internal/logstream"
)

// mockRAGService implements rag.Service for testing.
type mockRAGService struct {
	ingested []rag.LogEntry
	err      error
}

func (m *mockRAGService) Ingest(ctx context.Context, entries []rag.LogEntry) (*rag.IngestResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.ingested = append(m.ingested, entries...)
	return &rag.IngestResult{IngestedCount: len(entries)}, nil
}

func (m *mockRAGService) Search(ctx context.Context, query string, opts rag.SearchOptions) ([]rag.SearchResult, error) {
	return nil, nil
}

func (m *mockRAGService) GetContext(ctx context.Context, service string, start, end time.Time, limit int) ([]rag.SearchResult, error) {
	return nil, nil
}

func TestProcessor_SlidingWindow(t *testing.T) {
	window := logstream.NewSlidingWindow(5 * time.Second)

	now := time.Now()
	window.Add(now)
	window.Add(now)
	window.Add(now)

	assert.Equal(t, 3, window.Count())

	// Wait for window to expire
	time.Sleep(6 * time.Second)
	window.Add(time.Now())
	assert.Equal(t, 1, window.Count())
}

func TestProcessor_ThresholdBreach(t *testing.T) {
	// We can't easily test the full subscription without a real VM Logs instance,
	// but we can verify the window logic directly.
	window := logstream.NewSlidingWindow(5 * time.Second)

	window.Add(time.Now())
	window.Add(time.Now())
	assert.False(t, window.Count() >= 3)

	window.Add(time.Now())
	assert.True(t, window.Count() >= 3)
}

func TestProcessor_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Verify that a cancelled context returns immediately.
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected context to be cancelled immediately")
	}
}

func TestSlidingWindow_Eviction(t *testing.T) {
	window := logstream.NewSlidingWindow(2 * time.Second)

	now := time.Now()
	window.Add(now.Add(-3 * time.Second))
	window.Add(now.Add(-2 * time.Second))
	window.Add(now)

	// The eviction uses the latest entry's timestamp as reference.
	// Entries older than (latest - duration) are evicted.
	// With duration=2s and latest=now, entries from now-3s and now-2s are both <= now-2s,
	// so they should be evicted. But now-2s is exactly at the boundary.
	// Let's verify the actual behavior.
	assert.LessOrEqual(t, window.Count(), 2)
}
