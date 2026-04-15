package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// LokiTool queries logs from a single Grafana Loki endpoint.
type LokiTool struct {
	name    string
	baseURL string
	client  *http.Client
}

// NewLokiTool creates a tool backed by one Loki endpoint.
func NewLokiTool(name, baseURL string, client *http.Client) Tool {
	return &LokiTool{name: name, baseURL: baseURL, client: client}
}

func (t *LokiTool) Name() string { return "query_loki_logs" }

func (t *LokiTool) Description() string {
	return fmt.Sprintf("Query logs from Loki datasource %q using LogQL. Returns log entries with timestamps and labels.", t.name)
}

func (t *LokiTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "LogQL query expression"},
			"limit": {"type": "integer", "description": "Max log entries to return", "default": 50}
		},
		"required": ["query"]
	}`)
}

func (t *LokiTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}
	if params.Limit <= 0 {
		params.Limit = 50
	}

	u, _ := url.Parse(t.baseURL)
	u.Path = "/loki/api/v1/query_range"
	q := u.Query()
	q.Set("query", params.Query)
	q.Set("limit", fmt.Sprintf("%d", params.Limit))
	now := time.Now()
	q.Set("start", fmt.Sprintf("%d", now.Add(-time.Hour).UnixNano()))
	q.Set("end", fmt.Sprintf("%d", now.UnixNano()))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("loki query failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loki returned status %d: %s", resp.StatusCode, string(body))
	}

	entries, err := parseLokiResponse(body)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

// --- response types ---

type logEntry struct {
	Timestamp string            `json:"timestamp"`
	Labels    map[string]string `json:"labels"`
	Message   string            `json:"message"`
	Level     string            `json:"level,omitempty"`
}

func parseLokiResponse(body []byte) ([]logEntry, error) {
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Loki response: %w", err)
	}

	var entries []logEntry
	for _, r := range resp.Data.Result {
		for _, v := range r.Values {
			if len(v) < 2 {
				continue
			}
			entry := logEntry{
				Timestamp: v[0],
				Labels:    r.Stream,
				Message:   v[1],
			}
			if level, ok := r.Stream["level"]; ok {
				entry.Level = level
			}
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

var _ Tool = (*LokiTool)(nil)
