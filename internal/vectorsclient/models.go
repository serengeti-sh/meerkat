package vectorsclient

import "time"

// Entry represents a single log line for ingestion.
type Entry struct {
	ID         string
	Timestamp  time.Time
	Service    string
	Severity   string
	Body       string
	Attributes map[string]string
}

// IngestResult contains the outcome of an ingestion operation.
type IngestResult struct {
	IngestedCount     int
	DeduplicatedCount int
	FilteredCount     int
}

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
