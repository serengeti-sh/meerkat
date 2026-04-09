package victorialogs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/inspector/internal/datasource"
	"github.com/mandacode-labs/inspector/internal/datasource/provider/victorialogs"
)

func TestQueryLogs_JSONResponse(t *testing.T) {
	response := `[
		{"_time":"2024-01-01T00:00:00Z","_stream":"app:web","level":"error","_msg":"connection refused"},
		{"_time":"2024-01-01T00:00:01Z","_stream":"app:api","level":"info","_msg":"request completed"}
	]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/select/logsql/query", r.URL.Path)
		assert.Equal(t, "level:error", r.URL.Query().Get("query"))
		assert.Equal(t, "50", r.URL.Query().Get("limit"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	p := victorialogs.New("test", srv.URL)
	querier, ok := p.LogsQuerier()
	require.True(t, ok)

	entries, err := querier.QueryLogs(context.Background(), "level:error", 50)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, "connection refused", entries[0].Message)
	assert.Equal(t, "error", entries[0].Level)
	assert.Equal(t, "app:web", entries[0].Labels["_stream"])
}

func TestQueryLogs_JSONLResponse(t *testing.T) {
	response := `{"_time":"2024-01-01T00:00:00Z","_msg":"line1","level":"warn","_stream":"app:web"}
{"_time":"2024-01-01T00:00:01Z","_msg":"line2","level":"error","_stream":"app:api"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	p := victorialogs.New("test", srv.URL)
	querier, _ := p.LogsQuerier()

	entries, err := querier.QueryLogs(context.Background(), "*", 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, "line1", entries[0].Message)
	assert.Equal(t, "warn", entries[0].Level)
	assert.Equal(t, "line2", entries[1].Message)
}

func TestQueryLogs_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	p := victorialogs.New("test", srv.URL)
	querier, _ := p.LogsQuerier()

	entries, err := querier.QueryLogs(context.Background(), "nonexistent", 10)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestQueryLogs_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`query failed`))
	}))
	defer srv.Close()

	p := victorialogs.New("test", srv.URL)
	querier, _ := p.LogsQuerier()

	_, err := querier.QueryLogs(context.Background(), "*", 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestProvider_Interface(t *testing.T) {
	p := victorialogs.New("test", "http://localhost:9428")

	assert.Equal(t, "test", p.Name())
	assert.Equal(t, datasource.TypeVictoriaLogs, p.Type())

	_, hasMetrics := p.MetricsQuerier()
	assert.False(t, hasMetrics)

	_, hasLogs := p.LogsQuerier()
	assert.True(t, hasLogs)
}

func TestProvider_TestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/select/logsql/query", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := victorialogs.New("test", srv.URL)
	err := p.TestConnection(context.Background())
	require.NoError(t, err)
}
