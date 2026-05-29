package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/vectors"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

// TestMeerkatLogsEndToEnd demonstrates the complete MeerkatLogs pipeline flow:
// 1. Ingest log entries
// 2. Search for similar entries
// 3. Retrieve context for a time range
func TestMeerkatLogsEndToEnd(t *testing.T) {
	// Setup
	emb := &mockEmbedder{
		vectors: [][]float32{
			{0.1, 0.2, 0.3},
			{0.15, 0.25, 0.35},
			{0.8, 0.9, 1.0},
		},
	}
	vs := &inMemoryVectorStore{}
	svc, err := vectors.NewService(emb, vs)
	require.NoError(t, err)
	ctx := context.Background()

	// Step 1: Ingest log entries
	entries := []vectors.Entry{
		{
			ID:        "log-1",
			Timestamp: time.Now().Add(-30 * time.Minute),
			Service:   "api-server",
			Severity:  "ERROR",
			Body:      "connection refused to database",
		},
		{
			ID:        "log-2",
			Timestamp: time.Now().Add(-25 * time.Minute),
			Service:   "api-server",
			Severity:  "ERROR",
			Body:      "connection refused to redis cache",
		},
		{
			ID:        "log-3",
			Timestamp: time.Now().Add(-5 * time.Minute),
			Service:   "worker",
			Severity:  "INFO",
			Body:      "job completed successfully",
		},
	}

	result, err := svc.Ingest(ctx, entries)
	require.NoError(t, err)
	assert.Greater(t, result.IngestedCount, 0, "expected some entries to be ingested")

	// Step 2: Search for similar entries
	searchResults, err := svc.Search(ctx, "database connection error", vectors.SearchOptions{
		Limit:     10,
		TimeRange: time.Hour,
		Service:   "api-server",
		Severity:  "ERROR",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, searchResults, "expected search results")

	// Step 3: Get context for a specific time range
	now := time.Now()
	contextResults, err := svc.GetContext(ctx, "api-server", now.Add(-time.Hour), now, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, contextResults, "expected context results")
}

// mockEmbedder implements embed.Model for E2E testing.
type mockEmbedder struct {
	vectors [][]float32
	idx     int
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = m.vectors[m.idx%len(m.vectors)]
		m.idx++
	}
	return result, nil
}

// inMemoryVectorStore implements vector.VectorStore for E2E testing.
type inMemoryVectorStore struct {
	records []vectorstore.Record
}

func (m *inMemoryVectorStore) Insert(ctx context.Context, records []vectorstore.Record) error {
	m.records = append(m.records, records...)
	return nil
}

func (m *inMemoryVectorStore) Search(ctx context.Context, vector []float32, opts vectorstore.SearchOptions) ([]vectorstore.SearchResult, error) {
	// Simple stub: return all records
	results := make([]vectorstore.SearchResult, len(m.records))
	for i, r := range m.records {
		results[i] = vectorstore.SearchResult{
			ID:        r.ID,
			Body:      r.Body,
			Service:   r.Service,
			Severity:  r.Severity,
			Timestamp: r.Timestamp,
			Score:     0.9,
		}
	}
	return results, nil
}

func (m *inMemoryVectorStore) Delete(ctx context.Context, ids []string) error { return nil }
func (m *inMemoryVectorStore) Close() error                                   { return nil }
