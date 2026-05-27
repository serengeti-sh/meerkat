package analyzer

import (
	"github.com/serengeti-sh/meerkat/internal/tool"
)

// Tool is an interface for tools the LLM can invoke during the agent loop.
type Tool = tool.Interface

// ToolRegistry holds available tools and looks them up by name.
type ToolRegistry = tool.ToolRegistry

// ToolDef describes a tool for the LLM.
type ToolDef = tool.ToolDef

// NewToolRegistry creates a new tool registry with the given tools.
func NewToolRegistry(tools ...Tool) *ToolRegistry {
	return tool.NewToolRegistry(tools...)
}
