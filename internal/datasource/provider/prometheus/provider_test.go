package prometheus_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/datasource"
	"github.com/serengeti-sh/meerkat/internal/datasource/provider/prometheus"
)

func TestQueryMetrics_VectorResponse(t *testing.T) {
	// Real Prometheus/VM instant query response (vector)
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
		assert.Equal(t, "up", r.URL.Query().Get("query"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	p := prometheus.New("test", srv.URL)
	querier, ok := p.MetricsQuerier()
	require.True(t, ok)

	series, err := querier.QueryMetrics(context.Background(), "up")
	require.NoError(t, err)
	require.Len(t, series, 2)

	assert.Equal(t, "up", series[0].Labels["__name__"])
	assert.Equal(t, "node", series[0].Labels["job"])
	assert.InDelta(t, 1.0, series[0].Points[0].Value, 0.01)
	assert.InDelta(t, 0.0, series[1].Points[0].Value, 0.01)
}

func TestQueryMetrics_MatrixResponse(t *testing.T) {
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	p := prometheus.New("test", srv.URL)
	querier, _ := p.MetricsQuerier()

	series, err := querier.QueryMetrics(context.Background(), "rate")
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Len(t, series[0].Points, 3)
	assert.InDelta(t, 0.5, series[0].Points[0].Value, 0.01)
	assert.InDelta(t, 1.2, series[0].Points[1].Value, 0.01)
}

func TestQueryMetrics_EmptyResult(t *testing.T) {
	response := `{"status":"success","data":{"resultType":"vector","result":[]}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	p := prometheus.New("test", srv.URL)
	querier, _ := p.MetricsQuerier()

	series, err := querier.QueryMetrics(context.Background(), "nonexistent_metric")
	require.NoError(t, err)
	assert.Empty(t, series)
}

func TestQueryMetrics_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"status":"error","error":"query failed"}`))
	}))
	defer srv.Close()

	p := prometheus.New("test", srv.URL)
	querier, _ := p.MetricsQuerier()

	_, err := querier.QueryMetrics(context.Background(), "up")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestProvider_Interface(t *testing.T) {
	p := prometheus.New("test", "http://localhost:9090")

	assert.Equal(t, "test", p.Name())
	assert.Equal(t, datasource.TypePrometheus, p.Type())

	_, hasMetrics := p.MetricsQuerier()
	assert.True(t, hasMetrics)

	_, hasLogs := p.LogsQuerier()
	assert.False(t, hasLogs)
}

func TestProvider_TestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "up", r.URL.Query().Get("query"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := prometheus.New("test", srv.URL)
	err := p.TestConnection(context.Background())
	require.NoError(t, err)
}

func TestProvider_TestConnection_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	p := prometheus.New("test", srv.URL)
	err := p.TestConnection(context.Background())
	require.Error(t, err)
}

func TestQueryMetrics_ResultJSON(t *testing.T) {
	response := `{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [{"metric": {"__name__": "cpu"}, "value": [1700000000, "0.75"]}]
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(response))
	}))
	defer srv.Close()

	p := prometheus.New("test", srv.URL)
	querier, _ := p.MetricsQuerier()

	series, _ := querier.QueryMetrics(context.Background(), "cpu")
	data, _ := json.Marshal(series)

	assert.Contains(t, string(data), `"__name__":"cpu"`)
	assert.Contains(t, string(data), `"value":0.75`)
}
