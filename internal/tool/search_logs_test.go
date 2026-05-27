package tool_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/tool"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

type mockEmbedder struct {
	vectors [][]float32
	err     error
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.vectors, nil
}

type mockVectorStore struct {
	results []vectorstore.SearchResult
	err     error
}

func (m *mockVectorStore) Insert(ctx context.Context, records []vectorstore.Record) error {
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, vector []float32, opts vectorstore.SearchOptions) ([]vectorstore.SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func (m *mockVectorStore) Close() error { return nil }

func TestSearchLogsTool_Name(t *testing.T) {
	emb := &mockEmbedder{}
	vs := &mockVectorStore{}
	tool := tool.NewSearchLogsTool(emb, vs)
	assert.Equal(t, "search_logs", tool.Name())
}

func TestSearchLogsTool_Description(t *testing.T) {
	emb := &mockEmbedder{}
	vs := &mockVectorStore{}
	tool := tool.NewSearchLogsTool(emb, vs)
	assert.Contains(t, tool.Description(), "semantic")
}

func TestSearchLogsTool_Parameters(t *testing.T) {
	emb := &mockEmbedder{}
	vs := &mockVectorStore{}
	tool := tool.NewSearchLogsTool(emb, vs)

	params := tool.Parameters()
	var schema map[string]any
	require.NoError(t, json.Unmarshal(params, &schema))

	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "query")
	assert.Contains(t, props, "limit")
	assert.Contains(t, props, "time_range")
}

func TestSearchLogsTool_Execute(t *testing.T) {
	emb := &mockEmbedder{
		vectors: [][]float32{{0.1, 0.2, 0.3}},
	}
	vs := &mockVectorStore{
		results: []vectorstore.SearchResult{
			{
				ID:        "abc-123",
				Score:     0.95,
				Body:      "connection refused to database",
				Service:   "api-server",
				Severity:  "ERROR",
				Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	tool := tool.NewSearchLogsTool(emb, vs)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"database connection error"}`))
	require.NoError(t, err)

	var results []vectorstore.SearchResult
	require.NoError(t, json.Unmarshal([]byte(result), &results))
	require.Len(t, results, 1)
	assert.Equal(t, "connection refused to database", results[0].Body)
	assert.Equal(t, "api-server", results[0].Service)
	assert.Equal(t, "ERROR", results[0].Severity)
}

func TestSearchLogsTool_Execute_WithLimit(t *testing.T) {
	emb := &mockEmbedder{
		vectors: [][]float32{{0.1}},
	}
	vs := &mockVectorStore{
		results: []vectorstore.SearchResult{},
	}

	tool := tool.NewSearchLogsTool(emb, vs)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"test","limit":5,"time_range":"30m"}`))
	require.NoError(t, err)
	assert.Equal(t, "[]", result)
}

func TestSearchLogsTool_Execute_InvalidJSON(t *testing.T) {
	emb := &mockEmbedder{}
	vs := &mockVectorStore{}

	tool := tool.NewSearchLogsTool(emb, vs)
	_, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid parameters")
}

func TestSearchLogsTool_Execute_EmptyQuery(t *testing.T) {
	emb := &mockEmbedder{}
	vs := &mockVectorStore{}

	tool := tool.NewSearchLogsTool(emb, vs)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query":""}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

func TestSearchLogsTool_Execute_EmbedderError(t *testing.T) {
	emb := &mockEmbedder{err: assert.AnError}
	vs := &mockVectorStore{}

	tool := tool.NewSearchLogsTool(emb, vs)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"test"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embed query")
}

func TestSearchLogsTool_Execute_SearchError(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1}}}
	vs := &mockVectorStore{err: assert.AnError}

	tool := tool.NewSearchLogsTool(emb, vs)
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"test"}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "search logs")
}

func TestSearchLogsTool_Execute_DefaultTimeRange(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1}}}
	vs := &mockVectorStore{results: []vectorstore.SearchResult{}}

	tool := tool.NewSearchLogsTool(emb, vs)
	result, err := tool.Execute(context.Background(), json.RawMessage(`{"query":"test","time_range":"bad_duration"}`))
	require.NoError(t, err)
	assert.Equal(t, "[]", result)
}
