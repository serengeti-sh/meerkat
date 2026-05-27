package apperrors

import (
	"errors"
	"fmt"
	"net/http"
)

type ErrorType string

const (
	ErrNotFound       ErrorType = "not_found"
	ErrUnauthorized   ErrorType = "unauthorized"
	ErrInvalidInput   ErrorType = "invalid_input"
	ErrConflict       ErrorType = "conflict"
	ErrInternal       ErrorType = "internal_error"
	ErrForbidden      ErrorType = "forbidden"
	ErrNotImplemented ErrorType = "not_implemented"
	ErrUnavailable    ErrorType = "unavailable"
	ErrRateLimit      ErrorType = "rate_limit"
)

type appError struct {
	errorType ErrorType
	message   string
	cause     error
}

func New(errType ErrorType, message string) Error {
	return &appError{
		errorType: errType,
		message:   message,
	}
}

func Wrap(errType ErrorType, message string, cause error) Error {
	return &appError{
		errorType: errType,
		message:   message,
		cause:     cause,
	}
}

func (e *appError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

func (e *appError) Unwrap() error {
	return e.cause
}

func (e *appError) Type() ErrorType {
	return e.errorType
}

type Error interface {
	error
	Type() ErrorType
}

func Is(err error, target ErrorType) bool {
	var appErr Error
	if errors.As(err, &appErr) {
		return appErr.Type() == target
	}
	return false
}

func HTTPStatus(errType ErrorType) int {
	switch errType {
	case ErrNotFound:
		return http.StatusNotFound
	case ErrUnauthorized:
		return http.StatusUnauthorized
	case ErrInvalidInput:
		return http.StatusBadRequest
	case ErrConflict:
		return http.StatusConflict
	case ErrForbidden:
		return http.StatusForbidden
	case ErrNotImplemented:
		return http.StatusNotImplemented
	case ErrUnavailable:
		return http.StatusServiceUnavailable
	case ErrRateLimit:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}
