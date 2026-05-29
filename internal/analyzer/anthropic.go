package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// anthropicProvider implements LLMProvider for Anthropic Messages API.
type anthropicProvider struct {
	client      anthropic.Client
	model       string
	maxTokens   int64
	temperature float64
	retryCfg    RetryConfig
}

var _ LLMProvider = (*anthropicProvider)(nil)

func newAnthropicProvider(cfg ProviderConfig) LLMProvider {
	opts := []option.RequestOption{}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	baseURL := cfg.URL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	opts = append(opts, option.WithBaseURL(strings.TrimRight(baseURL, "/")))

	return &anthropicProvider{
		client:      anthropic.NewClient(opts...),
		model:       cfg.Model,
		maxTokens:   int64(cfg.MaxTokens),
		temperature: cfg.Temperature,
		retryCfg:    cfg.Retry,
	}
}

func (p *anthropicProvider) HealthCheck(ctx context.Context) error {
	_, err := p.Complete(ctx, &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		if errors.Is(err, ErrAuthError) {
			return nil
		}
		return fmt.Errorf("anthropic provider health check failed: %w", err)
	}
	return nil
}

func (p *anthropicProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	return retryWithBackoff(ctx, p.retryCfg, func() (*CompletionResponse, error) {
		var systemContent string
		messages := make([]anthropic.MessageParam, 0, len(req.Messages))

		for _, m := range req.Messages {
			switch m.Role {
			case RoleSystem:
				systemContent = m.Content
			case RoleTool:
				messages = append(messages, anthropic.NewUserMessage(
					anthropic.NewToolResultBlock(m.ToolCallID, m.Content, false),
				))
			case RoleAssistant:
				blocks := make([]anthropic.ContentBlockParamUnion, 0, 1+len(m.ToolCalls))
				if m.Content != "" {
					blocks = append(blocks, anthropic.NewTextBlock(m.Content))
				}
				for _, tc := range m.ToolCalls {
					var input any
					if err := json.Unmarshal(tc.Arguments, &input); err != nil {
						input = map[string]any{}
					}
					blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, input, tc.Name))
				}
				messages = append(messages, anthropic.NewAssistantMessage(blocks...))
			default:
				messages = append(messages, anthropic.NewUserMessage(
					anthropic.NewTextBlock(m.Content),
				))
			}
		}

		params := anthropic.MessageNewParams{
			Model:     p.model,
			MaxTokens: p.maxTokens,
			Messages:  messages,
		}

		if p.temperature > 0 {
			params.Temperature = anthropic.Float(p.temperature)
		}

		if systemContent != "" {
			params.System = []anthropic.TextBlockParam{
				{Text: systemContent},
			}
		}

		if len(req.Tools) > 0 {
			tools := make([]anthropic.ToolUnionParam, 0, len(req.Tools))
			for _, t := range req.Tools {
				schema, err := parseSchema(t.Parameters)
				if err != nil {
					return nil, err
				}
				tools = append(tools, anthropic.ToolUnionParam{
					OfTool: &anthropic.ToolParam{
						Name:        t.Name,
						Description: anthropic.String(t.Description),
						InputSchema: schema,
					},
				})
			}
			params.Tools = tools
		}

		msg, err := p.client.Messages.New(ctx, params)
		if err != nil {
			return nil, classifyAnthropicError(err)
		}

		out := &CompletionResponse{
			Stop: msg.StopReason == "end_turn",
		}

		for _, block := range msg.Content {
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
	})
}

func classifyAnthropicError(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 401, 403:
			return fmt.Errorf("%w: %s", ErrAuthError, err)
		case 400:
			raw := strings.ToLower(apiErr.RawJSON())
			if strings.Contains(raw, "prompt is too long") ||
				strings.Contains(raw, "too many tokens") {
				return fmt.Errorf("%w: %s", ErrContextOverflow, err)
			}
			return fmt.Errorf("%w: %s", ErrInvalidRequest, err)
		}
	}
	return err
}

func parseSchema(data json.RawMessage) (anthropic.ToolInputSchemaParam, error) {
	m, err := jsonToMap(data)
	if err != nil {
		return anthropic.ToolInputSchemaParam{}, err
	}
	schema := anthropic.ToolInputSchemaParam{}
	if props, ok := m["properties"]; ok {
		schema.Properties = props
	}
	if req, ok := m["required"].([]any); ok {
		required := make([]string, 0, len(req))
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
		schema.Required = required
	}
	return schema, nil
}
