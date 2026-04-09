package inspector

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mandacode-labs/inspector/internal/datasource"
)

// QueryMetricsTool queries metrics from a configured datasource via PromQL/LogQL.
type QueryMetricsTool struct {
	registry *datasource.Registry
}

func NewQueryMetricsTool(registry *datasource.Registry) *QueryMetricsTool {
	return &QueryMetricsTool{registry: registry}
}

func (t *QueryMetricsTool) Name() string { return "query_metrics" }
func (t *QueryMetricsTool) Description() string {
	return "Query metrics using PromQL. Parameters: datasource_name (string, name of a configured datasource), query (string, PromQL expression). Returns time series data."
}
func (t *QueryMetricsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"datasource_name": {"type": "string", "description": "Name of the configured datasource"},
			"query": {"type": "string", "description": "PromQL query expression"}
		},
		"required": ["datasource_name", "query"]
	}`)
}

func (t *QueryMetricsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		DatasourceName string `json:"datasource_name"`
		Query          string `json:"query"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	p, err := t.registry.Get(params.DatasourceName)
	if err != nil {
		return "", err
	}

	querier, ok := p.MetricsQuerier()
	if !ok {
		return "", fmt.Errorf("datasource %q does not support metrics queries", params.DatasourceName)
	}

	series, err := querier.QueryMetrics(ctx, params.Query)
	if err != nil {
		return "", fmt.Errorf("metrics query failed: %w", err)
	}

	data, err := json.Marshal(series)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

// QueryLogsTool queries logs from a configured datasource.
type QueryLogsTool struct {
	registry *datasource.Registry
}

func NewQueryLogsTool(registry *datasource.Registry) *QueryLogsTool {
	return &QueryLogsTool{registry: registry}
}

func (t *QueryLogsTool) Name() string { return "query_logs" }
func (t *QueryLogsTool) Description() string {
	return "Query logs using the datasource's query language. Parameters: datasource_name (string, name of a configured datasource), query (string, query expression), limit (integer, default 50). Returns log entries."
}
func (t *QueryLogsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"datasource_name": {"type": "string", "description": "Name of the configured datasource"},
			"query": {"type": "string", "description": "Query expression (LogsQL, LogQL, etc.)"},
			"limit": {"type": "integer", "description": "Max entries to return", "default": 50}
		},
		"required": ["datasource_name", "query"]
	}`)
}

func (t *QueryLogsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		DatasourceName string `json:"datasource_name"`
		Query          string `json:"query"`
		Limit          int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}

	p, err := t.registry.Get(params.DatasourceName)
	if err != nil {
		return "", err
	}

	querier, ok := p.LogsQuerier()
	if !ok {
		return "", fmt.Errorf("datasource %q does not support logs queries", params.DatasourceName)
	}

	entries, err := querier.QueryLogs(ctx, params.Query, params.Limit)
	if err != nil {
		return "", fmt.Errorf("logs query failed: %w", err)
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}
