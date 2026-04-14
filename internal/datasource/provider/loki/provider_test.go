package loki_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/datasource"
	"github.com/serengeti-sh/meerkat/internal/datasource/provider/loki"
)

func TestQueryLogs_Success(t *testing.T) {
	response := `{
		"status": "success",
		"data": {
			"resultType": "streams",
			"result": [
				{
					"stream": {"app": "web", "level": "error"},
					"values": [
						["1700000000000000000", "connection refused"],
						["1700000001000000000", "timeout exceeded"]
					]
				},
				{
					"stream": {"app": "api", "level": "info"},
					"values": [
						["1700000002000000000", "request completed"]
					]
				}
			]
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/loki/api/v1/query_range", r.URL.Path)
		assert.Equal(t, `{app="web"}`, r.URL.Query().Get("query"))
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	p := loki.New("test", srv.URL, http.DefaultClient)
	querier, ok := p.LogsQuerier()
	require.True(t, ok)

	entries, err := querier.QueryLogs(context.Background(), `{app="web"}`, 50)
	require.NoError(t, err)
	require.Len(t, entries, 3)

	assert.Equal(t, "connection refused", entries[0].Message)
	assert.Equal(t, "error", entries[0].Level)
	assert.Equal(t, "web", entries[0].Labels["app"])
	assert.Equal(t, "timeout exceeded", entries[1].Message)
	assert.Equal(t, "request completed", entries[2].Message)
	assert.Equal(t, "info", entries[2].Level)
}

func TestQueryLogs_EmptyResponse(t *testing.T) {
	response := `{"status":"success","data":{"resultType":"streams","result":[]}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	p := loki.New("test", srv.URL, http.DefaultClient)
	querier, _ := p.LogsQuerier()

	entries, err := querier.QueryLogs(context.Background(), `{app="nonexistent"}`, 10)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestQueryLogs_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`query failed`))
	}))
	defer srv.Close()

	p := loki.New("test", srv.URL, http.DefaultClient)
	querier, _ := p.LogsQuerier()

	_, err := querier.QueryLogs(context.Background(), `{app="test"}`, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestProvider_Interface(t *testing.T) {
	p := loki.New("test", "http://localhost:3100", http.DefaultClient)

	assert.Equal(t, "test", p.Name())
	assert.Equal(t, datasource.TypeLoki, p.Type())

	_, hasMetrics := p.MetricsQuerier()
	assert.False(t, hasMetrics)

	_, hasLogs := p.LogsQuerier()
	assert.True(t, hasLogs)
}

func TestProvider_TestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/loki/api/v1/labels", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := loki.New("test", srv.URL, http.DefaultClient)
	err := p.TestConnection(context.Background())
	require.NoError(t, err)
}

func TestProvider_TestConnection_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := loki.New("test", srv.URL, http.DefaultClient)
	err := p.TestConnection(context.Background())
	require.Error(t, err)
}
