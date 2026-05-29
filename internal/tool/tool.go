package tool

import (
	"context"
	"encoding/json"
)

// Plugin is the contract for tools the LLM can invoke during the agent loop.
type Plugin interface {
	// Name returns the tool identifier.
	Name() string

	// Description tells the LLM what this tool does.
	Description() string

	// Parameters returns a JSON Schema describing the tool's parameters.
	Parameters() json.RawMessage

	// Execute runs the tool with the given parameters and returns the result.
	Execute(ctx context.Context, args json.RawMessage) (string, error)
}

// Def describes a tool for the LLM.
type Def struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// Registry holds available tools and looks them up by name.
type Registry struct {
	tools map[string]Plugin
}

// NewRegistry creates a new tool registry with the given tools.
func NewRegistry(tools ...Plugin) *Registry {
	tr := &Registry{tools: make(map[string]Plugin)}
	for _, t := range tools {
		tr.tools[t.Name()] = t
	}
	return tr
}

func (r *Registry) Get(name string) (Plugin, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) All() []Plugin {
	result := make([]Plugin, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *Registry) Defs() []Def {
	defs := make([]Def, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, Def{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}
