package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

const (
	defaultMaxToolResultChars = 30000
	defaultMaxContextMessages = 50
)

// AnalyzerService defines the AI analysis service with agentic loop.
type AnalyzerService interface {
	Analyze(ctx context.Context, input *AnalysisInput) (*AnalysisResult, error)
}

// ServiceConfig holds configuration for the analyzer service.
type ServiceConfig struct {
	MaxIterations       int
	SystemPrompt        string
	MaxToolResultChars  int  // max characters per tool result (default 30000)
	SummarizeOnOverflow bool // enable conversation summarization on context overflow (default true)
	MaxContextMessages  int  // max messages before proactive trimming (default 50)
}

type service struct {
	provider       LLMProvider
	toolRegistry   *ToolRegistry
	maxIterations  int
	systemPrompt   string
	maxToolResult  int
	summarize      bool
	maxContextMsgs int
}

func NewService(provider LLMProvider, toolRegistry *ToolRegistry, cfg ServiceConfig) *service {
	if cfg.MaxToolResultChars == 0 {
		cfg.MaxToolResultChars = defaultMaxToolResultChars
	}
	if cfg.MaxContextMessages == 0 {
		cfg.MaxContextMessages = defaultMaxContextMessages
	}
	return &service{
		provider:       provider,
		toolRegistry:   toolRegistry,
		maxIterations:  cfg.MaxIterations,
		systemPrompt:   cfg.SystemPrompt,
		maxToolResult:  cfg.MaxToolResultChars,
		summarize:      cfg.SummarizeOnOverflow,
		maxContextMsgs: cfg.MaxContextMessages,
	}
}

// Analyze runs the agentic loop: call LLM, execute tools, repeat until done.
func (s *service) Analyze(ctx context.Context, input *AnalysisInput) (*AnalysisResult, error) {
	messages := s.buildInitialMessages(input)
	tools := s.toolRegistry.Defs()

	var totalIterations int

	for i := 0; i < s.maxIterations; i++ {
		totalIterations = i + 1

		resp, err := s.provider.Complete(ctx, &CompletionRequest{
			Messages: messages,
			Tools:    tools,
		})

		if err != nil {
			if errors.Is(err, ErrContextOverflow) && s.summarize {
				recovered := s.tryRecoverContext(&messages)
				if !recovered {
					return nil, fmt.Errorf("LLM call %d failed: context overflow, unable to reduce conversation size", i+1)
				}
				log.Printf("[meerkat] context overflow detected, summarized conversation, retrying")
				i--
				continue
			}
			return nil, fmt.Errorf("LLM call %d failed: %w", i+1, err)
		}

		// LLM finished without requesting tools
		if resp.Stop || len(resp.ToolCalls) == 0 {
			result, err := s.parseFinalResponse(resp, totalIterations)
			if err != nil {
				return nil, err
			}
			result.RawMessages = messages
			return result, nil
		}

		// Add assistant response with tool calls to conversation
		messages = append(messages, Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and append results
		for _, tc := range resp.ToolCalls {
			tool, ok := s.toolRegistry.Get(tc.Name)
			if !ok {
				messages = append(messages, Message{
					Role:       "tool",
					Content:    fmt.Sprintf("Error: unknown tool %q", tc.Name),
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
				})
				continue
			}

			log.Printf("[meerkat] tool call #%d: %s(%s)", i+1, tc.Name, string(tc.Arguments))

			result, err := tool.Execute(ctx, tc.Arguments)
			if err != nil {
				// Categorize and format tool errors so the LLM can reason about them
				result = formatToolError(tc.Name, tc.Arguments, err)
			}

			// Truncate oversized tool results
			result = s.truncateToolResult(result)

			messages = append(messages, Message{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
			})
		}

		log.Printf("[meerkat] iteration %d complete, continuing...", i+1)
	}

	// Max iterations reached — force a final response without tools
	resp, err := s.provider.Complete(ctx, &CompletionRequest{
		Messages: messages,
		// No tools — force final answer
	})
	if err != nil {
		return nil, fmt.Errorf("final LLM call failed: %w", err)
	}

	result, err := s.parseFinalResponse(resp, totalIterations)
	if err != nil {
		return nil, err
	}
	result.RawMessages = messages
	return result, nil
}

// formatToolError categorizes a tool execution error and returns a structured
// message for the LLM. It helps the LLM distinguish between parameter errors,
// connection failures, and query errors so it can decide whether to retry,
// try another datasource, or report failure honestly.
func formatToolError(toolName string, args json.RawMessage, err error) string {
	errStr := err.Error()

	// Categorize by common error patterns
	category := "execution"
	switch {
	case strings.Contains(errStr, "invalid parameters") || strings.Contains(errStr, "jsonschema"):
		category = "parameter_validation"
	case strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "no such host") || strings.Contains(errStr, "timeout"):
		category = "connection"
	case strings.Contains(errStr, "metrics query failed") || strings.Contains(errStr, "query failed") || strings.Contains(errStr, "returned status"):
		category = "query"
	}

	return fmt.Sprintf(
		"TOOL_ERROR [tool=%s category=%s]: %v\n\n"+
			"Arguments: %s\n\n"+
			"Do NOT guess or assume data based on this error. "+
			"If this is a connection error, you may try another datasource. "+
			"If this is a parameter validation error, fix the arguments and retry. "+
			"If all datasources fail, report the failure honestly without speculation.",
		toolName, category, err, string(args),
	)
}
