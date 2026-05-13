package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
)

// mockOpenAIServer simulates an OpenAI-compatible API.
type mockOpenAIServer struct {
	server *httptest.Server
	Calls  []map[string]any
}

func newMockOpenAIServer() *mockOpenAIServer {
	m := &mockOpenAIServer{
		Calls: make([]map[string]any, 0),
	}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			var reqBody map[string]any
			_ = json.NewDecoder(r.Body).Decode(&reqBody)
			m.Calls = append(m.Calls, reqBody)

			resp := map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role":    "assistant",
							"content": `{"severity":"critical","summary":"Error spike detected","detail":"High error rate in the last hour."}`,
						},
						"finish_reason": "stop",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/v1/embeddings" {
			resp := map[string]any{
				"object": "list",
				"data": []map[string]any{
					{
						"object":    "embedding",
						"index":     0,
						"embedding": make([]float64, 1536),
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	return m
}

func (m *mockOpenAIServer) URL() string {
	return m.server.URL
}

func (m *mockOpenAIServer) Close() {
	m.server.Close()
}

// mockPrometheusServer simulates a Prometheus-compatible API.
type mockPrometheusServer struct {
	server  *httptest.Server
	Queries []string
}

func newMockPrometheusServer() *mockPrometheusServer {
	m := &mockPrometheusServer{
		Queries: make([]string, 0),
	}
	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/query" {
			query := r.URL.Query().Get("query")
			m.Queries = append(m.Queries, query)

			resp := map[string]any{
				"status": "success",
				"data": map[string]any{
					"resultType": "vector",
					"result": []map[string]any{
						{
							"metric": map[string]string{"app": "api"},
							"value":  []any{1700000000, "99.5"},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	return m
}

func (m *mockPrometheusServer) URL() string {
	return m.server.URL
}

func (m *mockPrometheusServer) Close() {
	m.server.Close()
}
