package analyzer

import "errors"

// Sentinel errors that providers wrap around SDK errors
// so the service can classify them without importing SDK types.

var (
	// ErrContextOverflow indicates the conversation exceeds the model's context window.
	ErrContextOverflow = errors.New("context length exceeded")

	// ErrAuthError indicates an authentication or permission failure (401/403).
	ErrAuthError = errors.New("authentication error")

	// ErrInvalidRequest indicates a malformed request (400, not context_length_exceeded).
	ErrInvalidRequest = errors.New("invalid request")
)
