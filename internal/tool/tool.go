package tool

import (
	"context"
	"encoding/json"
)

// Tool is an interface for tools the LLM can invoke during the agent loop.
type Tool interface {
	// Name returns the tool identifier.
	Name() string

	// Description tells the LLM what this tool does.
	Description() string

	// Parameters returns a JSON Schema describing the tool's parameters.
	Parameters() json.RawMessage

	// Execute runs the tool with the given parameters and returns the result.
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolDef describes a tool for the LLM.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// ToolRegistry holds available tools and looks them up by name.
type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry(tools ...Tool) *ToolRegistry {
	tr := &ToolRegistry{tools: make(map[string]Tool)}
	for _, t := range tools {
		tr.tools[t.Name()] = t
	}
	return tr
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *ToolRegistry) Defs() []ToolDef {
	defs := make([]ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}
