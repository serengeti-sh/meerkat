// Package httphandler exposes the Meerkat REST API over HTTP.
//
// It maps incoming requests to inspect.Service methods and translates domain
// errors (errs.Error) to appropriate HTTP status codes.
package httphandler
