package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/metrics"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

const (
	defaultIngestBatchSize = 100
	defaultSearchLimit     = 10
	maxSearchLimit         = 100
)

// severityRank defines the priority order for severity levels.
// Higher index = more severe.
var severityRank = map[string]int{
	"debug":     0,
	"info":      1,
	"notice":    2,
	"warning":   3,
	"warn":      3,
	"error":     4,
	"err":       4,
	"critical":  5,
	"crit":      5,
	"alert":     6,
	"emergency": 7,
	"fatal":     7,
	"panic":     7,
}

// ServiceOption configures the RAG service.
type ServiceOption func(*service)

// WithSimilarityThreshold sets the template extraction similarity threshold.
func WithSimilarityThreshold(threshold float64) ServiceOption {
	return func(s *service) {
		if threshold > 0 {
			s.extractor.threshold = threshold
		}
	}
}

// WithBatchSize sets the ingestion batch size.
func WithBatchSize(size int) ServiceOption {
	return func(s *service) {
		if size > 0 {
			s.batchSize = size
		}
	}
}

// WithFilterMode configures log filtering during ingestion.
// mode: "all" (no filtering), "severity" (filter by minSeverity), "template" (deduplicate).
func WithFilterMode(mode, minSeverity string) ServiceOption {
	return func(s *service) {
		s.filterMode = strings.ToLower(mode)
		s.minSeverity = strings.ToLower(minSeverity)
	}
}

type service struct {
	embedder    embedder.Model
	vectorStore vectorstore.Store
	extractor   *Extractor
	batchSize   int
	filterMode  string
	minSeverity string
}

var _ Service = (*service)(nil)

// NewService creates a Service with the given dependencies.
func NewService(emb embedder.Model, vstore vectorstore.Store, opts ...ServiceOption) (*service, error) {
	if emb == nil {
		return nil, fmt.Errorf("rag: embedder is required")
	}
	if vstore == nil {
		return nil, fmt.Errorf("rag: vectorStore is required")
	}

	s := &service{
		embedder:    emb,
		vectorStore: vstore,
		extractor:   NewExtractor(),
		batchSize:   defaultIngestBatchSize,
		filterMode:  "template", // default: deduplicate by template
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

func (s *service) Ingest(ctx context.Context, entries []LogEntry) (*IngestResult, error) {
	if len(entries) == 0 {
		return &IngestResult{}, nil
	}

	var (
		ingested     int
		filtered     int
		deduplicated int
		records      []vectorstore.Record
	)

	minRank := severityRank[s.minSeverity]

	for _, entry := range entries {
		// Severity filtering.
		if s.filterMode == "severity" && s.minSeverity != "" {
			entryRank := severityRank[strings.ToLower(entry.Severity)]
			if entryRank < minRank {
				filtered++
				continue
			}
		}

		// Template deduplication.
		if s.filterMode == "template" {
			template, isNew := s.extractor.Extract(entry.Body)
			if !isNew {
				deduplicated++
				metrics.IngestDedupTotal.Inc()
				continue
			}
			entry.Body = template
		}

		records = append(records, vectorstore.NewRecord(
			nil, // vector will be set after embedding
			entry.Timestamp,
			entry.Service,
			entry.Severity,
			entry.Body,
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
	metrics.IngestTotal.WithLabelValues("success").Add(float64(ingested))

	return &IngestResult{
		IngestedCount:     ingested,
		DeduplicatedCount: deduplicated,
	}, nil
}

func (s *service) Search(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	start := time.Now()
	defer func() {
		metrics.SearchDuration.Observe(time.Since(start).Seconds())
	}()

	if query == "" {
		metrics.SearchTotal.WithLabelValues("error_empty_query").Inc()
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
		metrics.SearchTotal.WithLabelValues("error_embed").Inc()
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
		metrics.SearchTotal.WithLabelValues("error_store").Inc()
		return nil, fmt.Errorf("search vector store: %w", err)
	}

	if len(results) == 0 {
		metrics.SearchTotal.WithLabelValues("no_results").Inc()
		return nil, ErrNoResults
	}

	metrics.SearchTotal.WithLabelValues("success").Inc()
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
