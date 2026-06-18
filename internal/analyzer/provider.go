package analyzer

import (
	"context"

	"github.com/rs/zerolog"
)

// LLMProvider is the interface for LLM API calls.
type LLMProvider interface {
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// HealthCheck verifies connectivity to the LLM provider.
	HealthCheck(ctx context.Context) error
}

type ProviderConfig struct {
	Provider    string // openai (default, generic), anthropic
	URL         string
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
	Retry       RetryConfig
	Log         zerolog.Logger
}
