package rag

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

const (
	defaultIngestBatchSize = 100
	defaultSearchLimit     = 10
	maxSearchLimit         = 100
)

type service struct {
	embedder    embedder.Embedder
	vectorStore vectorstore.Store
	extractor   *Extractor
	batchSize   int
}

var _ Service = (*service)(nil)

// NewService creates a Service with the given dependencies.
func NewService(emb embedder.Embedder, vstore vectorstore.Store) (*service, error) {
	if emb == nil {
		return nil, fmt.Errorf("rag: embedder is required")
	}
	if vstore == nil {
		return nil, fmt.Errorf("rag: vectorStore is required")
	}

	return &service{
		embedder:    emb,
		vectorStore: vstore,
		extractor:   NewExtractor(),
		batchSize:   defaultIngestBatchSize,
	}, nil
}

func (s *service) Ingest(ctx context.Context, entries []LogEntry) (*IngestResult, error) {
	if len(entries) == 0 {
		return &IngestResult{}, nil
	}

	var (
		ingested     int
		deduplicated int
		records      []vectorstore.Record
	)

	for _, entry := range entries {
		template, isNew := s.extractor.Extract(entry.Body)
		if !isNew {
			deduplicated++
			continue
		}

		records = append(records, vectorstore.NewRecord(
			nil, // vector will be set after embedding
			entry.Timestamp,
			entry.Service,
			entry.Severity,
			template,
			entry.Attributes,
		))
	}

	if len(records) == 0 {
		return &IngestResult{
			IngestedCount:     ingested,
			DeduplicatedCount: deduplicated,
		}, nil
	}

	texts := make([]string, len(records))
	for i, r := range records {
		texts[i] = r.Body
	}

	vectors, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embed log templates: %w", err)
	}

	if len(vectors) != len(records) {
		return nil, fmt.Errorf("embedding count mismatch: got %d, want %d", len(vectors), len(records))
	}

	for i := range records {
		records[i].Vector = vectors[i]
		records[i].ID = uuid.New().String()
	}

	if err := s.vectorStore.Insert(ctx, records); err != nil {
		return nil, fmt.Errorf("insert records: %w", err)
	}

	ingested = len(records)

	return &IngestResult{
		IngestedCount:     ingested,
		DeduplicatedCount: deduplicated,
	}, nil
}

func (s *service) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, ErrEmptyQuery
	}

	if opts.Limit <= 0 {
		opts.Limit = defaultSearchLimit
	}
	if opts.Limit > maxSearchLimit {
		opts.Limit = maxSearchLimit
	}

	vectors, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	vsOpts := vectorstore.SearchOptions{
		Limit:     opts.Limit,
		TimeRange: opts.TimeRange,
		Service:   opts.Service,
		Severity:  opts.Severity,
	}

	results, err := s.vectorStore.Search(ctx, vectors[0], vsOpts)
	if err != nil {
		return nil, fmt.Errorf("search vector store: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrNoResults
	}

	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{
			ID:        r.ID,
			Score:     r.Score,
			Body:      r.Body,
			Service:   r.Service,
			Severity:  r.Severity,
			Timestamp: r.Timestamp,
		}
	}

	return out, nil
}

func (s *service) GetContext(ctx context.Context, service string, start, end time.Time, limit int) ([]SearchResult, error) {
	if start.After(end) {
		return nil, ErrInvalidTimeRange
	}

	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}

	timeRange := end.Sub(start)
	if timeRange <= 0 {
		return nil, ErrInvalidTimeRange
	}

	// Empty query yields a zero vector, which is equidistant to all records.
	// Combined with the service/time metadata filters, this returns the most
	// recent records for the service without requiring a semantic query.
	vectors, err := s.embedder.Embed(ctx, []string{""})
	if err != nil {
		return nil, fmt.Errorf("embed context query: %w", err)
	}

	vsOpts := vectorstore.SearchOptions{
		Limit:     limit,
		TimeRange: timeRange,
		Service:   service,
	}

	results, err := s.vectorStore.Search(ctx, vectors[0], vsOpts)
	if err != nil {
		return nil, fmt.Errorf("search context: %w", err)
	}

	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{
			ID:        r.ID,
			Score:     r.Score,
			Body:      r.Body,
			Service:   r.Service,
			Severity:  r.Severity,
			Timestamp: r.Timestamp,
		}
	}

	return out, nil
}
