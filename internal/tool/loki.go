package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// LokiTool queries logs from a single Grafana Loki endpoint.
type LokiTool struct {
	baseTool
	baseURL string
	client  *http.Client
}

// NewLokiTool creates a tool backed by one Loki endpoint.
func NewLokiTool(name, description, paramSchemaFile, baseURL string, client *http.Client) (Plugin, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if baseURL == "" {
		return nil, fmt.Errorf("loki tool %q: url is required", name)
	}

	base, err := newBaseTool(name, description, paramSchemaFile)
	if err != nil {
		return nil, err
	}

	return &LokiTool{baseTool: base, baseURL: baseURL, client: client}, nil
}

func (t *LokiTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	now := time.Now()
	const defaultLogLimit = 50
	q, err := argsToQueryParams(t.schema, args, url.Values{
		"limit": {strconv.Itoa(defaultLogLimit)},
		"start": {fmt.Sprintf("%d", now.Add(-time.Hour).UnixNano())},
		"end":   {fmt.Sprintf("%d", now.UnixNano())},
	})
	if err != nil {
		return "", err
	}

	u, _ := url.Parse(t.baseURL)
	u.Path = "/loki/api/v1/query_range"
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

var _ Plugin = (*LokiTool)(nil)
