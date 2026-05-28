// Package httphandler exposes the Meerkat REST API over HTTP.
//
// It maps incoming requests to inspector.Service methods and translates domain
// errors (apperrors.Error) to appropriate HTTP status codes.
package httphandler
