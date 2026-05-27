package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	summaryTruncateLen = 200
	resultPreviewLen   = 150
)

// Message represents a message in the LLM conversation.
type Message struct {
	Role       string     `json:"role"` // system, user, assistant, tool
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"` // assistant messages with tool calls
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolName   string     `json:"tool_name,omitempty"`
}

// ToolCall represents an LLM request to use a tool.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// CompletionRequest is sent to the LLM provider.
type CompletionRequest struct {
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools,omitempty"`
}

// CompletionResponse is returned from the LLM provider.
type CompletionResponse struct {
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Stop      bool       `json:"stop"` // true = LLM is done analyzing
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
	const defaultKeepExchanges = 2
	keepCount := defaultKeepExchanges
	if len(exchanges)-keepCount <= 0 {
		return false
	}
	cutExchanges := exchanges[:len(exchanges)-keepCount]

	// Build summary of removed exchanges
	var summaryParts []string
	for _, ex := range cutExchanges {
		summaryParts = append(summaryParts, summarizeExchange(ex))
	}

	var summaryMsg strings.Builder
	summaryMsg.WriteString("[CONVERSATION HISTORY SUMMARY]\n")
	summaryMsg.WriteString("Earlier investigation steps were summarized to save context:\n")
	for _, part := range summaryParts {
		summaryMsg.WriteString("- " + part + "\n")
	}

	// Reconstruct: system + summary + kept exchanges
	newMsgs := make([]Message, 0, len(msgs))
	newMsgs = append(newMsgs, msgs[0]) // system prompt
	newMsgs = append(newMsgs, Message{
		Role:    "user",
		Content: summaryMsg.String(),
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
				parts = append(parts, "Assistant: "+truncate(m.Content, summaryTruncateLen))
			}
			for _, tc := range m.ToolCalls {
				parts = append(parts, fmt.Sprintf("Called tool %q", tc.Name))
			}
		case "tool":
			result := m.Content
			if len(result) > resultPreviewLen {
				result = result[:resultPreviewLen] + "..."
			}
			parts = append(parts, fmt.Sprintf("Tool %q result: %s", m.ToolName, result))
		}
	}
	return strings.Join(parts, "; ")
}
