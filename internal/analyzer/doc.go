// Package analyzer orchestrates AI-driven root-cause analysis using an agentic loop.
//
// The analyzer service coordinates LLM calls, tool execution, conversation
// management, retry logic, and context-overflow recovery. It consumes data
// from datasources (Prometheus, Loki, VictoriaLogs) and produces structured
// AnalysisResult values.
//
// Entry point: NewService(provider, toolRegistry, cfg)
package analyzer
