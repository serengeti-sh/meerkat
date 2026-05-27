// Package apperrors defines domain-specific error types with HTTP status mapping.
//
// It provides typed errors (ErrNotFound, ErrInvalidInput, etc.), wrapping
// via Wrap(), and HTTP status code translation for use by the HTTP handler layer.
package apperrors
