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

// anthropicProvider implements LLMProvider for Anthropic Messages API.
type anthropicProvider struct {
	apiKey      string
	model       string
	baseURL     string
	maxTokens   int
	temperature float64
}

func newAnthropicProvider(cfg ProviderConfig) LLMProvider {
	baseURL := cfg.URL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &anthropicProvider{
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		baseURL:     strings.TrimRight(baseURL, "/"),
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
	}
}

func (p *anthropicProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	var systemContent string
	messages := make([]map[string]any, 0, len(req.Messages))

	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			systemContent = m.Content
		case "tool":
			// Anthropic uses tool_result content blocks
			messages = append(messages, map[string]any{
				"role": "user",
				"content": []map[string]any{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
		default:
			messages = append(messages, map[string]any{
				"role":    m.Role,
				"content": m.Content,
			})
		}
	}

	body := map[string]any{
		"model":       p.model,
		"max_tokens":  p.maxTokens,
		"temperature": p.temperature,
		"messages":    messages,
	}

	if systemContent != "" {
		body["system"] = systemContent
	}

	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, map[string]any{
				"name":         t.Name,
				"description":  t.Description,
				"input_schema": json.RawMessage(t.Parameters),
			})
		}
		body["tools"] = tools
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	out := &CompletionResponse{
		Stop: result.StopReason == "end_turn",
	}

	for _, block := range result.Content {
		switch block.Type {
		case "text":
			out.Content += block.Text
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
			out.Stop = false
		}
	}

	return out, nil
}
