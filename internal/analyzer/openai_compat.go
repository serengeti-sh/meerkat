package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

// openaiCompatProvider is a generic OpenAI-compatible API client.
// Works with: OpenAI, Ollama, vLLM, Groq, Together AI, Mistral, LiteLLM, etc.
type openaiCompatProvider struct {
	client      openai.Client
	model       string
	maxTokens   int64
	temperature float64
}

func newOpenAICompatProvider(cfg ProviderConfig) LLMProvider {
	opts := []option.RequestOption{}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.URL != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimRight(cfg.URL, "/")+"/v1"))
	}

	return &openaiCompatProvider{
		client:      openai.NewClient(opts...),
		model:       cfg.Model,
		maxTokens:   int64(cfg.MaxTokens),
		temperature: cfg.Temperature,
	}
}

func (p *openaiCompatProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages))
	for _, m := range req.Messages {
		messages = append(messages, toOpenAIMessage(m))
	}

	params := openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    p.model,
	}

	if p.maxTokens > 0 {
		params.MaxTokens = openai.Int(p.maxTokens)
	}
	if p.temperature > 0 {
		params.Temperature = openai.Float(p.temperature)
	}

	if len(req.Tools) > 0 {
		tools := make([]openai.ChatCompletionToolUnionParam, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Description),
				Parameters:  shared.FunctionParameters(jsonToMap(t.Parameters)),
			}))
		}
		params.Tools = tools
	}

	completion, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("API error: %w", err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	choice := completion.Choices[0]
	out := &CompletionResponse{
		Content: choice.Message.Content,
		Stop:    choice.FinishReason == "stop" || choice.FinishReason == "end_turn",
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

func toOpenAIMessage(m Message) openai.ChatCompletionMessageParamUnion {
	switch m.Role {
	case "system":
		return openai.SystemMessage(m.Content)
	case "user":
		return openai.UserMessage(m.Content)
	case "tool":
		return openai.ToolMessage(m.Content, m.ToolCallID)
	case "assistant":
		if len(m.ToolCalls) > 0 {
			toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
					OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
						ID: tc.ID,
						Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
							Name:      tc.Name,
							Arguments: string(tc.Arguments),
						},
					},
				})
			}
			return openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					Content:   openai.ChatCompletionAssistantMessageParamContentUnion{OfString: openai.String(m.Content)},
					ToolCalls: toolCalls,
				},
			}
		}
		return openai.AssistantMessage(m.Content)
	default:
		return openai.UserMessage(m.Content)
	}
}

// jsonToMap converts JSON bytes to a map for SDK parameters.
func jsonToMap(data json.RawMessage) map[string]any {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{"type": "object"}
	}
	return m
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
