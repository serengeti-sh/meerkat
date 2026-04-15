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

func TestVictoriaLogsTool_Execute_JSON(t *testing.T) {
	response := `[{"_time":"2024-01-01T00:00:00Z","_stream":"app:api","level":"error","_msg":"connection refused"}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/select/logsql/query", r.URL.Path)
		assert.Equal(t, "error", r.URL.Query().Get("query"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	v := tool.NewVictoriaLogsTool("test-vl", srv.URL, http.DefaultClient)

	assert.Equal(t, "test-vl", v.Name())

	result, err := v.Execute(context.Background(), json.RawMessage(`{"query": "error"}`))
	require.NoError(t, err)

	var entries []any
	require.NoError(t, json.Unmarshal([]byte(result), &entries))
	require.Len(t, entries, 1)

	e0 := entries[0].(map[string]any)
	assert.Equal(t, "connection refused", e0["message"])
	assert.Equal(t, "error", e0["level"])
}

func TestVictoriaLogsTool_Execute_JSONL(t *testing.T) {
	response := `{"_time":"2024-01-01T00:00:00Z","_msg":"line1","level":"info"}
{"_time":"2024-01-01T00:00:01Z","_msg":"line2","level":"warn"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	v := tool.NewVictoriaLogsTool("test", srv.URL, http.DefaultClient)

	result, err := v.Execute(context.Background(), json.RawMessage(`{"query": "*"}`))
	require.NoError(t, err)

	var entries []any
	require.NoError(t, json.Unmarshal([]byte(result), &entries))
	require.Len(t, entries, 2)

	assert.Equal(t, "line1", entries[0].(map[string]any)["message"])
	assert.Equal(t, "line2", entries[1].(map[string]any)["message"])
}

func TestVictoriaLogsTool_Execute_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	v := tool.NewVictoriaLogsTool("test", srv.URL, http.DefaultClient)

	_, err := v.Execute(context.Background(), json.RawMessage(`{"query": "test"}`))
	require.Error(t, err)
}

func TestVictoriaLogsTool_Execute_InvalidParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	v := tool.NewVictoriaLogsTool("test", srv.URL, http.DefaultClient)

	_, err := v.Execute(context.Background(), json.RawMessage(`not json`))
	require.Error(t, err)
}
