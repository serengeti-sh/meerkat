// Package tool defines the tool interface and registry used by the analyzer's
// agentic loop, and implements concrete tool integrations.
//
// Each tool (Prometheus, Loki, VictoriaLogs, Custom HTTP, SearchLogs, SearchRAG)
// implements the Plugin interface so the LLM can discover and invoke them dynamically.
// The Registry collects tools and exposes their JSON Schema definitions.
//
// Entry point:
//
//	reg := tool.NewRegistry(promTool, lokiTool, ...)
package tool
