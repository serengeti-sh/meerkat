package analyzer

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"context overflow is not retryable", fmt.Errorf("%w: too long", ErrContextOverflow), false},
		{"auth error is not retryable", fmt.Errorf("%w: bad key", ErrAuthError), false},
		{"invalid request is not retryable", fmt.Errorf("%w: bad request", ErrInvalidRequest), false},
		{"generic error is retryable", fmt.Errorf("network timeout"), true},
		{"wrapped generic error is retryable", fmt.Errorf("wrapped: %w", errors.New("connection refused")), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.retryable, isRetryable(tt.err))
		})
	}
}

func TestRetryWithBackoff_Success(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond}
	callCount := 0

	resp, err := retryWithBackoff(context.Background(), cfg, func() (*CompletionResponse, error) {
		callCount++
		return &CompletionResponse{Content: "ok", Stop: true}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, 1, callCount)
}

func TestRetryWithBackoff_RetriesOnTransientError(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond}
	callCount := 0

	resp, err := retryWithBackoff(context.Background(), cfg, func() (*CompletionResponse, error) {
		callCount++
		if callCount < 3 {
			return nil, fmt.Errorf("server error (502)")
		}
		return &CompletionResponse{Content: "recovered", Stop: true}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "recovered", resp.Content)
	assert.Equal(t, 3, callCount)
}

func TestRetryWithBackoff_NoRetryOnContextOverflow(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond}
	callCount := 0

	_, err := retryWithBackoff(context.Background(), cfg, func() (*CompletionResponse, error) {
		callCount++
		return nil, fmt.Errorf("%w: prompt too long", ErrContextOverflow)
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrContextOverflow)
	assert.Equal(t, 1, callCount) // no retries
}

func TestRetryWithBackoff_NoRetryOnAuthError(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, BaseDelay: 10 * time.Millisecond}
	callCount := 0

	_, err := retryWithBackoff(context.Background(), cfg, func() (*CompletionResponse, error) {
		callCount++
		return nil, fmt.Errorf("%w: invalid api key", ErrAuthError)
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAuthError)
	assert.Equal(t, 1, callCount)
}

func TestRetryWithBackoff_ExhaustsRetries(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 2, BaseDelay: 10 * time.Millisecond}
	callCount := 0

	_, err := retryWithBackoff(context.Background(), cfg, func() (*CompletionResponse, error) {
		callCount++
		return nil, fmt.Errorf("persistent error")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "after 2 retries")
	assert.Equal(t, 3, callCount) // initial + 2 retries
}

func TestRetryWithBackoff_RespectsContextCancellation(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 5, BaseDelay: 50 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err := retryWithBackoff(ctx, cfg, func() (*CompletionResponse, error) {
		callCount++
		return nil, fmt.Errorf("server error")
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRetryWithBackoff_ZeroRetries(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 0, BaseDelay: 10 * time.Millisecond}
	callCount := 0

	_, err := retryWithBackoff(context.Background(), cfg, func() (*CompletionResponse, error) {
		callCount++
		return nil, fmt.Errorf("fail immediately")
	})

	require.Error(t, err)
	assert.Equal(t, 1, callCount)
}
