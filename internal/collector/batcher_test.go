package collector

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

type mockEmbedder struct {
	mu      sync.Mutex
	calls   [][]string
	vectors [][]float32
	err     error
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, texts)
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		if i < len(m.vectors) {
			result[i] = m.vectors[i]
		} else {
			result[i] = []float32{0.1, 0.2, 0.3}
		}
	}
	return result, nil
}

type mockVectorStore struct {
	mu      sync.Mutex
	records []vectorstore.Record
	err     error
}

func (m *mockVectorStore) Insert(ctx context.Context, records []vectorstore.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.records = append(m.records, records...)
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, vector []float32, opts vectorstore.SearchOptions) ([]vectorstore.SearchResult, error) {
	return nil, nil
}

func (m *mockVectorStore) Delete(ctx context.Context, ids []string) error { return nil }

func (m *mockVectorStore) Close() error { return nil }

func testBatcherConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Collector.BatchSize = 3
	cfg.Collector.FlushInterval = 10 * time.Second
	return cfg
}

func TestBatcher_Add_FlushOnBatchSize(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1}, {0.2}, {0.3}}}
	vs := &mockVectorStore{}

	b := NewBatcher(testBatcherConfig(), emb, vs)

	entries := []LogEntry{
		{Body: "log1", Service: "svc", Severity: "info"},
		{Body: "log2", Service: "svc", Severity: "info"},
		{Body: "log3", Service: "svc", Severity: "info"},
	}

	for _, e := range entries {
		err := b.Add(e)
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		vs.mu.Lock()
		defer vs.mu.Unlock()
		return len(vs.records) == 3
	}, 2*time.Second, 50*time.Millisecond, "expected 3 records to be flushed")

	vs.mu.Lock()
	defer vs.mu.Unlock()
	assert.Len(t, vs.records, 3)
	assert.Equal(t, "log1", vs.records[0].Body)
	assert.Equal(t, "log3", vs.records[2].Body)
}

func TestBatcher_Stop_FlushesRemaining(t *testing.T) {
	cfg := testBatcherConfig()
	cfg.Collector.BatchSize = 100

	emb := &mockEmbedder{vectors: [][]float32{{0.1}, {0.2}}}
	vs := &mockVectorStore{}

	b := NewBatcher(cfg, emb, vs)
	b.Start()

	err := b.Add(LogEntry{Body: "log1", Service: "svc", Severity: "info"})
	require.NoError(t, err)
	err = b.Add(LogEntry{Body: "log2", Service: "svc", Severity: "warn"})
	require.NoError(t, err)

	b.Stop(context.Background())

	vs.mu.Lock()
	defer vs.mu.Unlock()
	assert.Len(t, vs.records, 2)
}

func TestBatcher_EmptyFlush(t *testing.T) {
	emb := &mockEmbedder{}
	vs := &mockVectorStore{}

	b := NewBatcher(testBatcherConfig(), emb, vs)
	b.triggerFlush()

	vs.mu.Lock()
	defer vs.mu.Unlock()
	assert.Empty(t, vs.records)
}

func TestBatcher_InsertError(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1}, {0.2}, {0.3}}}
	vs := &mockVectorStore{err: assert.AnError}

	b := NewBatcher(testBatcherConfig(), emb, vs)

	entries := []LogEntry{
		{Body: "log1", Service: "svc", Severity: "info"},
		{Body: "log2", Service: "svc", Severity: "info"},
		{Body: "log3", Service: "svc", Severity: "info"},
	}

	for _, e := range entries {
		err := b.Add(e)
		require.NoError(t, err)
	}

	assert.NotPanics(t, func() {
		require.Eventually(t, func() bool {
			emb.mu.Lock()
			defer emb.mu.Unlock()
			return len(emb.calls) > 0
		}, 2*time.Second, 50*time.Millisecond)
	})
}

func TestBatcher_GeneratesID(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1}}}
	vs := &mockVectorStore{}

	cfg := testBatcherConfig()
	cfg.Collector.BatchSize = 1

	b := NewBatcher(cfg, emb, vs)

	err := b.Add(LogEntry{Body: "log1", Service: "svc", Severity: "info"})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		vs.mu.Lock()
		defer vs.mu.Unlock()
		return len(vs.records) > 0
	}, 2*time.Second, 50*time.Millisecond)

	vs.mu.Lock()
	defer vs.mu.Unlock()
	assert.NotEmpty(t, vs.records[0].ID, "ID should be auto-generated")
}

func TestBatcher_Add_AfterStop(t *testing.T) {
	emb := &mockEmbedder{}
	vs := &mockVectorStore{}

	b := NewBatcher(testBatcherConfig(), emb, vs)
	b.Start()
	b.Stop(context.Background())

	err := b.Add(LogEntry{Body: "log1", Service: "svc", Severity: "info"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stopped")
}
