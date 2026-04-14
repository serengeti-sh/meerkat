package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type service struct {
	provider      LLMProvider
	toolRegistry  *ToolRegistry
	maxIterations int
	systemPrompt  string
}

func NewService(provider LLMProvider, toolRegistry *ToolRegistry, maxIterations int, systemPrompt string) AnalyzerService {
	return &service{
		provider:      provider,
		toolRegistry:  toolRegistry,
		maxIterations: maxIterations,
		systemPrompt:  systemPrompt,
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
				result = fmt.Sprintf("Error executing tool %q: %v", tc.Name, err)
			}

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
