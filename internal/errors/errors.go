package errors

import "net/http"

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

type err struct {
	errorType ErrorType
	message   string
}

func New(errType ErrorType, message string) Error {
	return &err{
		errorType: errType,
		message:   message,
	}
}

func (e *err) Error() string {
	return e.message
}

func (e *err) Type() ErrorType {
	return e.errorType
}

type Error interface {
	error
	Type() ErrorType
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
