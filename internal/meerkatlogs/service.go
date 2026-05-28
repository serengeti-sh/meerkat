package meerkatlogs

import (
	"context"
	"time"
)

// Service provides log ingestion and semantic search for AI analysis.
type Service interface {
	// Ingest adds log entries to the vector store after template extraction.
	Ingest(ctx context.Context, entries []LogEntry) (*IngestResult, error)

	// Search finds semantically similar log entries.
	Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error)

	// GetContext retrieves relevant log context for a given service and time range.
	GetContext(ctx context.Context, service string, start, end time.Time, limit int) ([]SearchResult, error)
}

// LogEntry represents a single log line for ingestion.
type LogEntry struct {
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
