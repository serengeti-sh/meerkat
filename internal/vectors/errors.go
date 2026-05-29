package vectors

import "errors"

var (
	// ErrEmptyQuery indicates a search query was provided without content.
	ErrEmptyQuery = errors.New("search query is empty")

	// ErrNoResults indicates the search returned no matching records.
	ErrNoResults = errors.New("no matching records found")

	// ErrInvalidTimeRange indicates the requested time range is invalid.
	ErrInvalidTimeRange = errors.New("invalid time range")
)
