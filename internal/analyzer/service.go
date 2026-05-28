package analyzer

import "context"

// Service defines the AI analysis service with agentic loop.
type Service interface {
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
