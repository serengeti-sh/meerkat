package rag_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/rag"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

// mockEmbedder implements embedder.Model for testing.
type mockEmbedder struct {
	vectors [][]float32
	err     error
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		if i < len(m.vectors) {
			result[i] = m.vectors[i]
		} else {
			result[i] = []float32{float32(i) * 0.1}
		}
	}
	return result, nil
}

// mockVectorStore implements vectorstore.VectorStore for testing.
type mockVectorStore struct {
	records []vectorstore.Record
	results []vectorstore.SearchResult
	err     error
}

func (m *mockVectorStore) Insert(ctx context.Context, records []vectorstore.Record) error {
	if m.err != nil {
		return m.err
	}
	m.records = append(m.records, records...)
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, vector []float32, opts vectorstore.SearchOptions) ([]vectorstore.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockVectorStore) Delete(ctx context.Context, ids []string) error { return nil }
func (m *mockVectorStore) Close() error                                   { return nil }

func TestService_Ingest(t *testing.T) {
	tests := []struct {
		name         string
		entries      []rag.LogEntry
		wantErr      bool
		wantIngested int
		wantDedup    int
	}{
		{
			name:         "empty entries",
			entries:      nil,
			wantIngested: 0,
			wantDedup:    0,
		},
		{
			name: "single entry",
			entries: []rag.LogEntry{
				{Body: "connection refused", Service: "api", Severity: "ERROR"},
			},
			wantIngested: 1,
			wantDedup:    0,
		},
		{
			name: "duplicate entries",
			entries: []rag.LogEntry{
				{Body: "connection refused", Service: "api", Severity: "ERROR"},
				{Body: "connection refused", Service: "api", Severity: "ERROR"},
			},
			wantIngested: 1,
			wantDedup:    1,
		},
		{
			name: "different entries",
			entries: []rag.LogEntry{
				{Body: "connection refused", Service: "api", Severity: "ERROR"},
				{Body: "timeout occurred", Service: "api", Severity: "ERROR"},
			},
			wantIngested: 2,
			wantDedup:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emb := &mockEmbedder{vectors: [][]float32{{0.1}, {0.2}}}
			vs := &mockVectorStore{}
			svc, err := rag.NewService(emb, vs)
			require.NoError(t, err)

			result, err := svc.Ingest(context.Background(), tt.entries)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantIngested, result.IngestedCount)
			assert.Equal(t, tt.wantDedup, result.DeduplicatedCount)
		})
	}
}

func TestService_Search(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1}}}
	vs := &mockVectorStore{
		results: []vectorstore.SearchResult{
			{ID: "1", Body: "error: connection failed", Score: 0.95},
		},
	}
	svc, err := rag.NewService(emb, vs)
	require.NoError(t, err)

	t.Run("successful search", func(t *testing.T) {
		results, err := svc.Search(context.Background(), "connection error", rag.SearchOptions{Limit: 5})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "error: connection failed", results[0].Body)
	})

	t.Run("empty query", func(t *testing.T) {
		_, err := svc.Search(context.Background(), "", rag.SearchOptions{})
		require.ErrorIs(t, err, rag.ErrEmptyQuery)
	})

	t.Run("no results", func(t *testing.T) {
		emptyVS := &mockVectorStore{results: nil}
		emptySvc, err := rag.NewService(emb, emptyVS)
		require.NoError(t, err)
		_, err = emptySvc.Search(context.Background(), "query", rag.SearchOptions{})
		require.ErrorIs(t, err, rag.ErrNoResults)
	})
}

func TestService_GetContext(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1}}}
	vs := &mockVectorStore{
		results: []vectorstore.SearchResult{
			{ID: "1", Body: "error: connection failed", Service: "api", Timestamp: time.Now()},
		},
	}
	svc, err := rag.NewService(emb, vs)
	require.NoError(t, err)

	t.Run("successful context retrieval", func(t *testing.T) {
		now := time.Now()
		results, err := svc.GetContext(context.Background(), "api", now.Add(-time.Hour), now, 10)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.Equal(t, "api", results[0].Service)
	})

	t.Run("invalid time range", func(t *testing.T) {
		now := time.Now()
		_, err := svc.GetContext(context.Background(), "api", now, now.Add(-time.Hour), 10)
		require.ErrorIs(t, err, rag.ErrInvalidTimeRange)
	})
}

func TestService_Ingest_ErrorCases(t *testing.T) {
	t.Run("embedder error", func(t *testing.T) {
		emb := &mockEmbedder{err: assert.AnError}
		vs := &mockVectorStore{}
		svc, err := rag.NewService(emb, vs)
		require.NoError(t, err)

		entries := []rag.LogEntry{
			{Body: "error message", Service: "api", Severity: "ERROR"},
		}
		_, err = svc.Ingest(context.Background(), entries)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "embed")
	})

	t.Run("vector store error", func(t *testing.T) {
		emb := &mockEmbedder{vectors: [][]float32{{0.1}}}
		vs := &mockVectorStore{err: assert.AnError}
		svc, err := rag.NewService(emb, vs)
		require.NoError(t, err)

		entries := []rag.LogEntry{
			{Body: "error message", Service: "api", Severity: "ERROR"},
		}
		_, err = svc.Ingest(context.Background(), entries)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insert")
	})
}
