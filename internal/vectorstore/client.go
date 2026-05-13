package vectorstore

import (
	"context"
	"time"
)

// Record is a normalized vector record for insertion into Milvus.
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
	ID        string
	Score     float32
	Body      string
	Service   string
	Severity  string
	Timestamp time.Time
}

// VectorStore defines the interface for vector database operations.
type VectorStore interface {
	// Insert adds vector records to the store.
	Insert(ctx context.Context, records []Record) error

	// Search finds the most similar vectors to the given query vector.
	// limit controls the maximum number of results.
	// timeRange restricts results to records newer than Now() - timeRange.
	Search(ctx context.Context, vector []float32, limit int, timeRange time.Duration) ([]SearchResult, error)

	// Close releases resources held by the store.
	Close() error
}
