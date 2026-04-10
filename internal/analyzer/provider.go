package analyzer

import "context"

// LLMProvider is the interface for LLM API calls.
type LLMProvider interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
}

type ProviderConfig struct {
	Provider    string // openai (default, generic), anthropic
	URL         string
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
}
