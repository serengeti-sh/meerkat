package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openaiCompatProvider is a generic OpenAI-compatible API client.
// Works with: OpenAI, Ollama, vLLM, Groq, Together AI, Mistral, LiteLLM, etc.
type openaiCompatProvider struct {
	apiKey      string
	model       string
	baseURL     string
	maxTokens   int
	temperature float64
}

func newOpenAICompatProvider(cfg ProviderConfig) LLMProvider {
	return &openaiCompatProvider{
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		baseURL:     strings.TrimRight(cfg.URL, "/"),
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
	}
}

func (p *openaiCompatProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	messages := make([]map[string]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		msg := map[string]any{
			"role":    m.Role,
			"content": m.Content,
		}
		if m.ToolCallID != "" {
			msg["tool_call_id"] = m.ToolCallID
		}
		messages = append(messages, msg)
	}

	body := map[string]any{
		"model":       p.model,
		"messages":    messages,
		"max_tokens":  p.maxTokens,
		"temperature": p.temperature,
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.Name,
					"description": t.Description,
					"parameters":  json.RawMessage(t.Parameters),
				},
			})
		}
		body["tools"] = tools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
				FinishReason string `json:"finish_reason"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	choice := result.Choices[0]
	out := &CompletionResponse{
		Content: choice.Message.Content,
		Stop:    choice.Message.FinishReason == "stop" || choice.Message.FinishReason == "end_turn",
	}

	for _, tc := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
		out.Stop = false
	}

	return out, nil
}

// NewLLMProvider creates the appropriate provider based on config.
func NewLLMProvider(cfg ProviderConfig) LLMProvider {
	switch cfg.Provider {
	case "anthropic":
		return newAnthropicProvider(cfg)
	default:
		return newOpenAICompatProvider(cfg)
	}
}
