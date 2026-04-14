package analyzer

import (
	"context"
	"encoding/json"
	"time"
)

// Severity levels for analysis results.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// AnalysisInput is what goes into the agent loop.
type AnalysisInput struct {
	Trigger     string          `json:"trigger"` // manual, webhook, scheduled
	TriggerID   string          `json:"trigger_id"`
	Query       string          `json:"query"` // optional specific query
	Datasources []DatasourceRef `json:"datasources"`
	Context     string          `json:"context"` // additional context (e.g. webhook payload)
}

type DatasourceRef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// AnalysisResult is the final output of the agent loop.
type AnalysisResult struct {
	Severity    Severity  `json:"severity"`
	Summary     string    `json:"summary"`
	Detail      string    `json:"detail"`
	Datasources []string  `json:"datasources_used"`
	Iterations  int       `json:"iterations"`
	RawMessages []Message `json:"-"`
	CompletedAt time.Time `json:"completed_at"`
}

// AnalyzerService defines the AI analysis service with agentic loop.
type AnalyzerService interface {
	Analyze(ctx context.Context, input *AnalysisInput) (*AnalysisResult, error)
}

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

// ToolDef describes a tool for the LLM.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
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
