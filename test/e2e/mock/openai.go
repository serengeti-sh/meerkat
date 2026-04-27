package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
)

// OpenAICall records a single call to the mock OpenAI API.
type OpenAICall struct {
	Messages []any `json:"messages"`
	Tools    []any `json:"tools,omitempty"`
}

// MockOpenAI is a fake OpenAI-compatible API server for e2e tests.
// It simulates an agentic loop:
//   - 1st call: responds with a tool_call to query_metrics
//   - 2nd call: responds with final analysis JSON
type MockOpenAI struct {
	Server    *httptest.Server
	mu        sync.Mutex
	Calls     []OpenAICall
	callCount int
	responses []openaiResponseFn
}

// openaiResponseFn returns the response body for a given call index.
type openaiResponseFn func(callIdx int, reqBody map[string]any) map[string]any

// NewMockOpenAI creates a mock OpenAI server with a default 2-step agentic scenario.
func NewMockOpenAI() *MockOpenAI {
	m := &MockOpenAI{}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", m.handleCompletions)
	m.Server = httptest.NewServer(mux)

	// Default scenario: 1 tool call then final answer
	m.responses = []openaiResponseFn{
		// Step 1: request query_metrics tool call
		func(callIdx int, reqBody map[string]any) map[string]any {
			return map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role":    "assistant",
							"content": "I'll check the error metrics first.",
							"tool_calls": []map[string]any{
								{
									"id":   "call_tool_1",
									"type": "function",
								"function": map[string]any{
									"name":      "test-vm",
									"arguments": `{"query":"rate(http_errors_total[5m])"}`,
								},
								},
							},
						},
						"finish_reason": "tool_calls",
					},
				},
			}
		},
		// Step 2: final analysis
		func(callIdx int, reqBody map[string]any) map[string]any {
			return map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{
							"role":    "assistant",
							"content": `{"severity":"critical","summary":"Error spike detected in API server","detail":"HTTP 500 errors increased to 1523 req/s and HTTP 503 errors at 847 req/s. This indicates a significant degradation in the api-server service."}`,
						},
						"finish_reason": "stop",
					},
				},
			}
		},
	}

	return m
}

// SetResponses overrides the default response sequence.
func (m *MockOpenAI) SetResponses(responses ...openaiResponseFn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = responses
}

// URL returns the mock server base URL.
func (m *MockOpenAI) URL() string {
	return m.Server.URL
}

// Close shuts down the mock server.
func (m *MockOpenAI) Close() {
	m.Server.Close()
}

// Reset clears call history.
func (m *MockOpenAI) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = nil
	m.callCount = 0
}

func (m *MockOpenAI) handleCompletions(w http.ResponseWriter, r *http.Request) {
	var reqBody map[string]any
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	call := OpenAICall{
		Messages: reqBody["messages"].([]any),
	}
	if tools, ok := reqBody["tools"]; ok {
		call.Tools = tools.([]any)
	}
	m.Calls = append(m.Calls, call)

	idx := m.callCount
	m.callCount++
	responses := m.responses
	m.mu.Unlock()

	var resp map[string]any
	if idx < len(responses) {
		resp = responses[idx](idx, reqBody)
	} else {
		// Fallback: return a simple final answer
		resp = map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": `{"severity":"info","summary":"Analysis complete","detail":"No issues found."}`,
					},
					"finish_reason": "stop",
				},
			},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ErrorScenario returns a response function that returns an HTTP error.
func ErrorScenario(statusCode int, message string) openaiResponseFn {
	return func(callIdx int, reqBody map[string]any) map[string]any {
		// Return valid JSON that will cause the analyzer to handle it as non-parseable
		return map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": fmt.Sprintf("Error: %s", message),
					},
					"finish_reason": "stop",
				},
			},
		}
	}
}

// DirectAnswerScenario returns a response function that immediately answers without tool calls.
func DirectAnswerScenario(severity, summary, detail string) openaiResponseFn {
	return func(callIdx int, reqBody map[string]any) map[string]any {
		return map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": fmt.Sprintf(`{"severity":"%s","summary":"%s","detail":"%s"}`, severity, summary, detail),
					},
					"finish_reason": "stop",
				},
			},
		}
	}
}
