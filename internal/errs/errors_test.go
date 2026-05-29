package errs

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(ErrNotFound, "resource not found")
	assert.Equal(t, "resource not found", err.Error())
	assert.Equal(t, ErrNotFound, err.Type())
}

func TestWrap(t *testing.T) {
	cause := errors.New("db connection failed")
	err := Wrap(ErrInternal, "failed to create report", cause)
	assert.Contains(t, err.Error(), "failed to create report")
	assert.Contains(t, err.Error(), "db connection failed")
	assert.Equal(t, ErrInternal, err.Type())
	assert.ErrorIs(t, err, cause)
}

func TestIs(t *testing.T) {
	err := New(ErrNotFound, "not found")
	assert.True(t, Is(err, ErrNotFound))
	assert.False(t, Is(err, ErrInternal))
	assert.False(t, Is(errors.New("plain error"), ErrNotFound))
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		errType  ErrorType
		expected int
	}{
		{"not found", ErrNotFound, http.StatusNotFound},
		{"unauthorized", ErrUnauthorized, http.StatusUnauthorized},
		{"invalid input", ErrInvalidInput, http.StatusBadRequest},
		{"conflict", ErrConflict, http.StatusConflict},
		{"forbidden", ErrForbidden, http.StatusForbidden},
		{"not implemented", ErrNotImplemented, http.StatusNotImplemented},
		{"unavailable", ErrUnavailable, http.StatusServiceUnavailable},
		{"internal falls back to 500", ErrInternal, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, HTTPStatus(tt.errType))
		})
	}
}
