package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/serengeti-sh/meerkat/internal/rag"
)

// searchRAGTool searches the RAG pipeline for semantically similar log entries.
type searchRAGTool struct {
	ragSvc rag.Service
}

// NewSearchRAGTool creates a tool that searches the RAG index.
func NewSearchRAGTool(ragSvc rag.Service) Interface {
	return &searchRAGTool{ragSvc: ragSvc}
}

func (t *searchRAGTool) Name() string {
	return "search_rag"
}

func (t *searchRAGTool) Description() string {
	return "Search semantically similar log entries from the RAG (Retrieval-Augmented Generation) index. Returns log entries with timestamps, service names, severity levels, and raw body text."
}

func (t *searchRAGTool) Parameters() json.RawMessage {
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

func (t *searchRAGTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
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

	opts := rag.SearchOptions{
		Limit:     params.Limit,
		TimeRange: timeRange,
		Service:   params.Service,
		Severity:  params.Severity,
	}

	results, err := t.ragSvc.Search(ctx, params.Query, opts)
	if err != nil {
		return "", fmt.Errorf("search rag: %w", err)
	}

	out, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("marshal results: %w", err)
	}

	return string(out), nil
}

var _ Interface = (*searchRAGTool)(nil)
