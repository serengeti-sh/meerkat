package analyzer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"
)

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

func NewService(provider LLMProvider, toolRegistry *ToolRegistry, cfg ServiceConfig) AnalyzerService {
	if cfg.MaxToolResultChars == 0 {
		cfg.MaxToolResultChars = 30000
	}
	if cfg.MaxContextMessages == 0 {
		cfg.MaxContextMessages = 50
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

func (s *service) buildInitialMessages(input *AnalysisInput) []Message {
	dsInfo := "No datasources configured."
	if len(input.Datasources) > 0 {
		dsJSON, _ := json.Marshal(input.Datasources)
		dsInfo = string(dsJSON)
	}

	userMsg := fmt.Sprintf("## Trigger: %s (ID: %s)\n\n## Available Datasources:\n%s",
		input.Trigger, input.TriggerID, dsInfo)

	if input.Query != "" {
		userMsg += fmt.Sprintf("\n\n## Query:\n%s", input.Query)
	}
	if input.Context != "" {
		userMsg += fmt.Sprintf("\n\n## Additional Context:\n%s", input.Context)
	}

	return []Message{
		{Role: "system", Content: s.systemPrompt},
		{Role: "user", Content: userMsg},
	}
}

// truncateToolResult truncates oversized tool results to maxToolResult characters.
func (s *service) truncateToolResult(result string) string {
	if len(result) <= s.maxToolResult {
		return result
	}
	// Find a valid UTF-8 boundary
	cutoff := s.maxToolResult
	for cutoff > 0 && !utf8.RuneStart(result[cutoff]) {
		cutoff--
	}
	return result[:cutoff] + fmt.Sprintf(
		"\n\n[TRUNCATED: original %d chars, showing first %d]",
		len(result), cutoff,
	)
}

// tryRecoverContext attempts to reduce the conversation size by summarizing
// earlier exchanges. Returns true if messages were reduced, false if unrecoverable.
func (s *service) tryRecoverContext(messages *[]Message) bool {
	msgs := *messages

	// Need at least system + user + 1 assistant exchange to trim
	if len(msgs) <= 3 {
		return false
	}

	// Group messages into exchanges (assistant + following tool results)
	exchanges := groupExchanges(msgs[1:]) // skip system prompt
	if len(exchanges) <= 1 {
		return false
	}

	// Keep the last 2 exchanges, summarize the rest
	keepCount := 2
	if len(exchanges)-keepCount <= 0 {
		return false
	}
	cutExchanges := exchanges[:len(exchanges)-keepCount]

	// Build summary of removed exchanges
	var summaryParts []string
	for _, ex := range cutExchanges {
		summaryParts = append(summaryParts, summarizeExchange(ex))
	}

	summaryMsg := "[CONVERSATION HISTORY SUMMARY]\n"
	summaryMsg += "Earlier investigation steps were summarized to save context:\n"
	for _, part := range summaryParts {
		summaryMsg += "- " + part + "\n"
	}

	// Reconstruct: system + summary + kept exchanges
	newMsgs := make([]Message, 0, len(msgs))
	newMsgs = append(newMsgs, msgs[0]) // system prompt
	newMsgs = append(newMsgs, Message{
		Role:    "user",
		Content: summaryMsg,
	})

	for _, ex := range exchanges[len(exchanges)-keepCount:] {
		newMsgs = append(newMsgs, ex.messages...)
	}

	*messages = newMsgs
	return len(newMsgs) < len(msgs)
}

// exchange groups an assistant message with its subsequent tool result messages.
type exchange struct {
	messages []Message
}

func groupExchanges(msgs []Message) []exchange {
	var exchanges []exchange
	var current *exchange

	for _, m := range msgs {
		if m.Role == "assistant" {
			if current != nil {
				exchanges = append(exchanges, *current)
			}
			current = &exchange{}
		}
		if current != nil {
			current.messages = append(current.messages, m)
		}
	}
	if current != nil {
		exchanges = append(exchanges, *current)
	}
	return exchanges
}

func summarizeExchange(ex exchange) string {
	var parts []string
	for _, m := range ex.messages {
		switch m.Role {
		case "assistant":
			if m.Content != "" {
				parts = append(parts, "Assistant: "+truncate(m.Content, 200))
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, fmt.Sprintf("Called tool %q", tc.Name))
			}
		case "tool":
			result := m.Content
			if len(result) > 150 {
				result = result[:150] + "..."
			}
			parts = append(parts, fmt.Sprintf("Tool %q result: %s", m.ToolName, result))
		}
	}
	return strings.Join(parts, "; ")
}

func (s *service) parseFinalResponse(resp *CompletionResponse, iterations int) (*AnalysisResult, error) {
	content := resp.Content

	// Try to extract JSON from the response
	var parsed struct {
		Severity string `json:"severity"`
		Summary  string `json:"summary"`
		Detail   string `json:"detail"`
	}

	// Strip markdown code blocks if present
	content = stripCodeBlocks(content)

	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// If not valid JSON, use raw content as summary
		return &AnalysisResult{
			Severity:    SeverityInfo,
			Summary:     truncate(content, 500),
			Detail:      content,
			Iterations:  iterations,
			CompletedAt: time.Now(),
		}, nil
	}

	severity := Severity(parsed.Severity)
	switch severity {
	case SeverityWarning, SeverityCritical:
		// valid
	default:
		severity = SeverityInfo
	}

	return &AnalysisResult{
		Severity:    severity,
		Summary:     parsed.Summary,
		Detail:      parsed.Detail,
		Iterations:  iterations,
		CompletedAt: time.Now(),
	}, nil
}

func stripCodeBlocks(s string) string {
	if len(s) > 6 && s[:3] == "```" {
		// Find closing ```
		end := len(s) - 3
		if s[end:] == "```" {
			// Strip opening ```json or ``` and closing ```
			start := 3
			for start < len(s) && s[start] != '\n' {
				start++
			}
			if start < len(s) {
				start++ // skip newline
			}
			return s[start:end]
		}
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
