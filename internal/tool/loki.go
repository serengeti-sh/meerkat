package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// LokiTool queries logs from a single Grafana Loki endpoint.
type LokiTool struct {
	name        string
	description string
	params      json.RawMessage
	baseURL     string
	client      *http.Client
}

// NewLokiTool creates a tool backed by one Loki endpoint.
func NewLokiTool(name, description, paramSchemaFile, baseURL string, client *http.Client) (Tool, error) {
	if name == "" {
		return nil, fmt.Errorf("loki tool: name is required")
	}
	if description == "" {
		return nil, fmt.Errorf("loki tool %q: description is required", name)
	}
	if paramSchemaFile == "" {
		return nil, fmt.Errorf("loki tool %q: param_schema_file is required", name)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("loki tool %q: url is required", name)
	}

	params, err := os.ReadFile(paramSchemaFile)
	if err != nil {
		return nil, fmt.Errorf("loki tool %q: failed to read param schema %q: %w", name, paramSchemaFile, err)
	}

	return &LokiTool{name: name, description: description, params: json.RawMessage(params), baseURL: baseURL, client: client}, nil
}

func (t *LokiTool) Name() string { return t.name }

func (t *LokiTool) Description() string { return t.description }

func (t *LokiTool) Parameters() json.RawMessage { return t.params }

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
