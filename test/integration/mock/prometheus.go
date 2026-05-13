package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
)

// PrometheusResponse defines a canned Prometheus instant query response.
type PrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// MockPrometheus is a fake Prometheus/VM HTTP server for e2e tests.
type MockPrometheus struct {
	Server      *httptest.Server
	Queries     []string // captured queries
	responses   map[string]json.RawMessage
	defaultResp json.RawMessage
}

// NewMockPrometheus creates a mock Prometheus server with default error-spike data.
func NewMockPrometheus() *MockPrometheus {
	m := &MockPrometheus{
		responses: make(map[string]json.RawMessage),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", m.handleQuery)
	m.Server = httptest.NewServer(mux)

	// Default: error spike response
	m.defaultResp = errorSpikeResponse()
	return m
}

// SetResponse sets a canned response for a specific query string.
func (m *MockPrometheus) SetResponse(query string, resp any) {
	data, _ := json.Marshal(resp)
	m.responses[query] = data
}

// URL returns the mock server base URL.
func (m *MockPrometheus) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockPrometheus) Close() {
	m.Server.Close()
}

func (m *MockPrometheus) handleQuery(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	m.Queries = append(m.Queries, query)

	var resp json.RawMessage
	if canned, ok := m.responses[query]; ok {
		resp = canned
	} else {
		resp = m.defaultResp
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp)
}

// errorSpikeResponse returns a Prometheus-style response simulating an error spike.
func errorSpikeResponse() json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {"__name__": "http_errors_total", "job": "api-server", "status": "500"},
					"value": [%.1f, "1523"]
				},
				{
					"metric": {"__name__": "http_errors_total", "job": "api-server", "status": "503"},
					"value": [%.1f, "847"]
				}
			]
		}
	}`, float64(1710000000), float64(1710000000)))
}

// EmptyResponse returns a Prometheus-style response with no data.
func EmptyResponse() json.RawMessage {
	return json.RawMessage(`{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": []
		}
	}`)
}
