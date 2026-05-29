package vectorstore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Record is a normalized vector record for insertion into a vector store.
type Record struct {
	ID         string
	Vector     []float32
	Timestamp  time.Time
	Service    string
	Severity   string
	Body       string
	Attributes map[string]string
}

// SearchResult represents a single vector search match.
type SearchResult struct {
	ID         string
	Score      float32
	Body       string
	Service    string
	Severity   string
	Timestamp  time.Time
	Attributes map[string]string
}

// SearchOptions holds optional filters for vector search.
type SearchOptions struct {
	Limit     int
	TimeRange time.Duration
	Service   string
	Severity  string
}

// Store defines the interface for vector database operations.
type Store interface {
	// Insert adds vector records to the store.
	Insert(ctx context.Context, records []Record) error

	// Search finds the most similar vectors to the given query vector.
	Search(ctx context.Context, vector []float32, opts SearchOptions) ([]SearchResult, error)

	// Delete removes records by their IDs.
	Delete(ctx context.Context, ids []string) error

	// Ping checks connectivity to the vector store.
	Ping(ctx context.Context) error

	// Close releases resources held by the store.
	Close() error
}

// NewRecord creates a Record with a generated UUID ID.
func NewRecord(vector []float32, timestamp time.Time, service, severity, body string, attrs map[string]string) Record {
	return Record{
		ID:         uuid.New().String(),
		Vector:     vector,
		Timestamp:  timestamp,
		Service:    service,
		Severity:   severity,
		Body:       body,
		Attributes: attrs,
	}
}

// Validate checks if the record has all required fields.
func (r Record) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("record ID is required")
	}
	if len(r.Vector) == 0 {
		return fmt.Errorf("record vector is required")
	}
	if r.Body == "" {
		return fmt.Errorf("record body is required")
	}
	return nil
}
