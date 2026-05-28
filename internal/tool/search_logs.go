package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

// searchLogsTool performs semantic search over stored log entries.
type searchLogsTool struct {
	embedder    embedder.Model
	vectorstore vectorstore.Store
}

var _ Plugin = (*searchLogsTool)(nil)

func NewSearchLogsTool(emb embedder.Model, vstore vectorstore.Store) Plugin {
	return &searchLogsTool{
		embedder:    emb,
		vectorstore: vstore,
	}
}

func (t *searchLogsTool) Name() string {
	return "search_logs"
}

func (t *searchLogsTool) Description() string {
	return "Search semantically similar log entries using vector similarity. Returns log entries with timestamps, service names, severity levels, and raw body text."
}

func (t *searchLogsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Natural language query to search for similar logs"
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of results to return",
				"default": 10
			},
			"time_range": {
				"type": "string",
				"description": "Time range for search, e.g., '15m', '1h'",
				"default": "1h"
			},
			"service": {
				"type": "string",
				"description": "Filter by service name (optional)"
			},
			"severity": {
				"type": "string",
				"description": "Filter by severity level, e.g., 'ERROR', 'WARN' (optional)"
			}
		},
		"required": ["query"]
	}`)
}

func (t *searchLogsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query     string `json:"query"`
		Limit     int    `json:"limit"`
		TimeRange string `json:"time_range"`
		Service   string `json:"service"`
		Severity  string `json:"severity"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	if params.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	const (
		defaultSearchLimit = 10
		maxSearchLimit     = 100
	)
	if params.Limit <= 0 {
		params.Limit = defaultSearchLimit
	}
	if params.Limit > maxSearchLimit {
		params.Limit = maxSearchLimit
	}

	timeRange, err := time.ParseDuration(params.TimeRange)
	if err != nil {
		timeRange = time.Hour
	}

	vectors, err := t.embedder.Embed(ctx, []string{params.Query})
	if err != nil {
		return "", fmt.Errorf("embed query: %w", err)
	}

	opts := vectorstore.SearchOptions{
		Limit:     params.Limit,
		TimeRange: timeRange,
		Service:   params.Service,
		Severity:  params.Severity,
	}
	results, err := t.vectorstore.Search(ctx, vectors[0], opts)
	if err != nil {
		return "", fmt.Errorf("search logs: %w", err)
	}

	out, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("marshal results: %w", err)
	}

	return string(out), nil
}
