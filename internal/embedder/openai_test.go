package embedder

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIEmbedder_Embed(t *testing.T) {
	resp := map[string]any{
		"object": "list",
		"data": []any{
			map[string]any{
				"object":    "embedding",
				"index":     float64(0),
				"embedding": []float64{0.1, 0.2, 0.3},
			},
			map[string]any{
				"object":    "embedding",
				"index":     float64(1),
				"embedding": []float64{0.4, 0.5, 0.6},
			},
		},
		"model": "text-embedding-3-small",
		"usage": map[string]any{
			"prompt_tokens": float64(2),
			"total_tokens":  float64(2),
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/embeddings", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		var body map[string]any
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Contains(t, body, "input")
		assert.Equal(t, "text-embedding-3-small", body["model"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	e := newOpenAIEmbedder("test-key", srv.URL, "text-embedding-3-small")

	vectors, err := e.Embed(context.Background(), []string{"hello", "world"})
	require.NoError(t, err)
	require.Len(t, vectors, 2)

	assert.InDelta(t, 0.1, vectors[0][0], 0.001)
	assert.InDelta(t, 0.2, vectors[0][1], 0.001)
	assert.InDelta(t, 0.3, vectors[0][2], 0.001)
	assert.InDelta(t, 0.4, vectors[1][0], 0.001)
}

func TestOpenAIEmbedder_Embed_EmptyInput(t *testing.T) {
	e := newOpenAIEmbedder("test-key", "", "text-embedding-3-small")
	vectors, err := e.Embed(context.Background(), nil)
	require.NoError(t, err)
	assert.Nil(t, vectors)
}

func TestOpenAIEmbedder_Embed_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	e := newOpenAIEmbedder("test-key", srv.URL, "text-embedding-3-small")
	_, err := e.Embed(context.Background(), []string{"hello"})
	require.Error(t, err)
}

func TestOpenAIEmbedder_DefaultModel(t *testing.T) {
	var e Embedder = newOpenAIEmbedder("key", "", "")
	oe := e.(*openAIEmbedder)
	assert.Equal(t, "text-embedding-3-small", oe.model)
}

func TestNew_ReturnsOpenAI(t *testing.T) {
	e := New("key", "", "text-embedding-3-small")
	assert.NotNil(t, e)
}
