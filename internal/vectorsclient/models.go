package vectorsclient

import "time"

// SearchResult represents a single matching log entry.
type SearchResult struct {
	ID        string
	Score     float32
	Body      string
	Service   string
	Severity  string
	Timestamp time.Time
}

// SearchOptions holds optional filters for semantic search.
type SearchOptions struct {
	Limit     int
	TimeRange time.Duration
	Service   string
	Severity  string
}
