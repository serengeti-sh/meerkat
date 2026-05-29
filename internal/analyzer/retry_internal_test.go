package analyzer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, true},
		{"generic error", errors.New("some error"), true},
		{"context overflow", analyzer.ErrContextOverflow, false},
		{"auth error", analyzer.ErrAuthError, false},
		{"invalid request", analyzer.ErrInvalidRequest, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.ExportIsRetryable(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRetryWithBackoff_NoRetries(t *testing.T) {
	cfg := analyzer.RetryConfig{
		MaxRetries: 0,
		BaseDelay:  100 * time.Millisecond,
	}

	callCount := 0
	fn := func() (*analyzer.CompletionResponse, error) {
		callCount++
		return &analyzer.CompletionResponse{Content: "success"}, nil
	}

	resp, err := analyzer.ExportRetryWithBackoff(context.Background(), cfg, fn)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.Content)
	assert.Equal(t, 1, callCount)
}

func TestRetryWithBackoff_SuccessAfterRetries(t *testing.T) {
	cfg := analyzer.RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
	}

	callCount := 0
	fn := func() (*analyzer.CompletionResponse, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("temporary error")
		}
		return &analyzer.CompletionResponse{Content: "success"}, nil
	}

	resp, err := analyzer.ExportRetryWithBackoff(context.Background(), cfg, fn)
	require.NoError(t, err)
	assert.Equal(t, "success", resp.Content)
	assert.Equal(t, 3, callCount)
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	cfg := analyzer.RetryConfig{
		MaxRetries: 3,
		BaseDelay:  10 * time.Millisecond,
	}

	callCount := 0
	fn := func() (*analyzer.CompletionResponse, error) {
		callCount++
		return nil, analyzer.ErrAuthError
	}

	_, err := analyzer.ExportRetryWithBackoff(context.Background(), cfg, fn)
	require.Error(t, err)
	assert.Equal(t, analyzer.ErrAuthError, err)
	assert.Equal(t, 1, callCount)
}

func TestRetryWithBackoff_MaxRetriesExceeded(t *testing.T) {
	cfg := analyzer.RetryConfig{
		MaxRetries: 2,
		BaseDelay:  10 * time.Millisecond,
	}

	callCount := 0
	fn := func() (*analyzer.CompletionResponse, error) {
		callCount++
		return nil, errors.New("persistent error")
	}

	_, err := analyzer.ExportRetryWithBackoff(context.Background(), cfg, fn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "after 2 retries")
	assert.Equal(t, 3, callCount) // initial + 2 retries
}

func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
	cfg := analyzer.RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	fn := func() (*analyzer.CompletionResponse, error) {
		return nil, errors.New("error")
	}

	_, err := analyzer.ExportRetryWithBackoff(ctx, cfg, fn)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}
