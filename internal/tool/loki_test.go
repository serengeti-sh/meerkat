package tool_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/tool"
)

func TestLokiTool_Execute(t *testing.T) {
	response := `{
		"status": "success",
		"data": {
			"resultType": "streams",
			"result": [
				{
					"stream": {"level": "error", "app": "api"},
					"values": [["1700000000000000000", "connection refused"], ["1700000001000000000", "timeout"]]
				}
			]
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/loki/api/v1/query_range", r.URL.Path)
		assert.Equal(t, `{app="api"}`, r.URL.Query().Get("query"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	l, err := tool.NewLokiTool("test-loki", "test loki", srv.URL, http.DefaultClient)
	require.NoError(t, err)

	assert.Equal(t, "test-loki", l.Name())

	result, err := l.Execute(context.Background(), json.RawMessage(`{"query": "{app=\"api\"}"}`))
	require.NoError(t, err)

	var entries []any
	require.NoError(t, json.Unmarshal([]byte(result), &entries))
	require.Len(t, entries, 2)

	e0 := entries[0].(map[string]any)
	assert.Equal(t, "connection refused", e0["message"])
	assert.Equal(t, "error", e0["level"])
}

func TestLokiTool_Execute_WithLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[]}}`))
	}))
	defer srv.Close()

	l, err := tool.NewLokiTool("test", "test loki", srv.URL, http.DefaultClient)
	require.NoError(t, err)

	result, err := l.Execute(context.Background(), json.RawMessage(`{"query": "{app=\"api\"}", "limit": 10}`))
	require.NoError(t, err)
	assert.Equal(t, "null", result)
}

func TestLokiTool_Execute_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	l, err := tool.NewLokiTool("test", "test loki", srv.URL, http.DefaultClient)
	require.NoError(t, err)

	_, err = l.Execute(context.Background(), json.RawMessage(`{"query": "test"}`))
	require.Error(t, err)
}

func TestLokiTool_Execute_InvalidParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	l, err := tool.NewLokiTool("test", "test loki", srv.URL, http.DefaultClient)
	require.NoError(t, err)

	_, err = l.Execute(context.Background(), json.RawMessage(`not json`))
	require.Error(t, err)
}
