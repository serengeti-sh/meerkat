package analyzer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
)

// RetryConfig holds retry parameters for provider API calls.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
}

// isRetryable determines if an error should be retried.
func isRetryable(err error) bool {
	if errors.Is(err, ErrContextOverflow) ||
		errors.Is(err, ErrAuthError) ||
		errors.Is(err, ErrInvalidRequest) {
		return false
	}
	return true
}

// retryWithBackoff wraps a function call with exponential backoff and jitter.
func retryWithBackoff(ctx context.Context, cfg RetryConfig, fn func() (*CompletionResponse, error)) (*CompletionResponse, error) {
	if cfg.MaxRetries <= 0 {
		return fn()
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = cfg.BaseDelay
	bo.MaxInterval = 30 * time.Second
	bo.Multiplier = 2.0

	var resp *CompletionResponse
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		resp, lastErr = fn()
		if lastErr == nil {
			return resp, nil
		}
		if !isRetryable(lastErr) {
			return nil, lastErr
		}
		if attempt == cfg.MaxRetries {
			break
		}

		nextBackoff := bo.NextBackOff()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(nextBackoff):
			log.Printf("[meerkat] retry attempt %d/%d after %v: %v",
				attempt+1, cfg.MaxRetries, nextBackoff, lastErr)
		}
	}
	return nil, fmt.Errorf("after %d retries: %w", cfg.MaxRetries, lastErr)
}
