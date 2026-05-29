package analyzer

import "context"

// Export internal functions for testing only.

func ExportIsRetryable(err error) bool {
	return isRetryable(err)
}

func ExportRetryWithBackoff(ctx context.Context, cfg RetryConfig, fn func() (*CompletionResponse, error)) (*CompletionResponse, error) {
	return retryWithBackoff(ctx, cfg, fn)
}
