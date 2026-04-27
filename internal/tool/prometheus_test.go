package tool_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/tool"
)

// getQueryValue extracts the "query" parameter from either URL query or POST form.
func getQueryValue(r *http.Request) string {
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.FormValue("query")
}

func writePromResponse(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// writeSchemaFile creates a temporary file with the given schema content and returns its path.
func writeSchemaFile(t *testing.T, schema string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "schema.json")
	require.NoError(t, os.WriteFile(p, []byte(schema), 0644))
	return p
}

const promSchema = `{"type":"object","properties":{"query":{"type":"string","description":"PromQL query expression"}},"required":["query"]}`

func TestPrometheusTool_Execute_VectorResponse(t *testing.T) {
	response := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"__name__": "up", "job": "node", "instance": "localhost:9090"},
					"value": [1700000000, "1"]
				},
				{
					"metric": {"__name__": "up", "job": "node", "instance": "localhost:9091"},
					"value": [1700000000, "0"]
				}
			]
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/query", r.URL.Path)
		assert.Equal(t, "up", getQueryValue(r))
		writePromResponse(w, response)
	}))
	defer srv.Close()

	p, err := tool.NewPrometheusTool("test", "test prometheus", writeSchemaFile(t, promSchema), srv.URL, http.DefaultClient)
	require.NoError(t, err)

	assert.Equal(t, "test", p.Name())

	result, err := p.Execute(context.Background(), json.RawMessage(`{"query": "up"}`))
	require.NoError(t, err)

	var series []any
	require.NoError(t, json.Unmarshal([]byte(result), &series))
	require.Len(t, series, 2)

	s0 := series[0].(map[string]any)
	labels := s0["labels"].(map[string]any)
	assert.Equal(t, "up", labels["__name__"])
}

func TestPrometheusTool_Execute_MatrixResponse(t *testing.T) {
	response := `{
		"status": "success",
		"data": {
			"resultType": "matrix",
			"result": [
				{
					"metric": {"__name__": "rate"},
					"values": [[1700000000, "0.5"], [1700000060, "1.2"], [1700000120, "0.8"]]
				}
			]
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writePromResponse(w, response)
	}))
	defer srv.Close()

	p, err := tool.NewPrometheusTool("test", "test prometheus", writeSchemaFile(t, promSchema), srv.URL, http.DefaultClient)
	require.NoError(t, err)

	result, err := p.Execute(context.Background(), json.RawMessage(`{"query": "rate"}`))
	require.NoError(t, err)

	var series []any
	require.NoError(t, json.Unmarshal([]byte(result), &series))
	require.Len(t, series, 1)

	s0 := series[0].(map[string]any)
	points := s0["points"].([]any)
	assert.Len(t, points, 3)
}

func TestPrometheusTool_Execute_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writePromResponse(w, `{"status":"success","data":{"resultType":"vector","result":[]}}`)
	}))
	defer srv.Close()

	p, err := tool.NewPrometheusTool("test", "test prometheus", writeSchemaFile(t, promSchema), srv.URL, http.DefaultClient)
	require.NoError(t, err)

	result, err := p.Execute(context.Background(), json.RawMessage(`{"query": "nonexistent"}`))
	require.NoError(t, err)
	assert.Equal(t, "[]", result)
}

func TestPrometheusTool_Execute_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"error","errorType":"internal","error":"query failed"}`))
	}))
	defer srv.Close()

	p, err := tool.NewPrometheusTool("test", "test prometheus", writeSchemaFile(t, promSchema), srv.URL, http.DefaultClient)
	require.NoError(t, err)

	_, err = p.Execute(context.Background(), json.RawMessage(`{"query": "up"}`))
	require.Error(t, err)
}

func TestPrometheusTool_Execute_InvalidParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	p, err := tool.NewPrometheusTool("test", "test prometheus", writeSchemaFile(t, promSchema), srv.URL, http.DefaultClient)
	require.NoError(t, err)

	_, err = p.Execute(context.Background(), json.RawMessage(`not json`))
	require.Error(t, err)
}
