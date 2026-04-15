package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// VictoriaLogsTool queries logs from a single Victoria Logs endpoint.
type VictoriaLogsTool struct {
	name    string
	baseURL string
	client  *http.Client
}

// NewVictoriaLogsTool creates a tool backed by one Victoria Logs endpoint.
func NewVictoriaLogsTool(name, baseURL string, client *http.Client) Tool {
	return &VictoriaLogsTool{name: name, baseURL: baseURL, client: client}
}

func (t *VictoriaLogsTool) Name() string { return "query_victorialogs_logs" }

func (t *VictoriaLogsTool) Description() string {
	return fmt.Sprintf("Query logs from Victoria Logs datasource %q using LogsQL. Returns log entries.", t.name)
}

func (t *VictoriaLogsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "LogsQL query expression"},
			"limit": {"type": "integer", "description": "Max log entries to return", "default": 50}
		},
		"required": ["query"]
	}`)
}

func (t *VictoriaLogsTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
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
	u.Path = "/select/logsql/query"
	q := u.Query()
	q.Set("query", params.Query)
	q.Set("limit", fmt.Sprintf("%d", params.Limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("victoria logs query failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("victoria logs returned status %d: %s", resp.StatusCode, string(body))
	}

	entries, err := parseVictoriaLogsResponse(body)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

// vlLogEntry represents a log entry from Victoria Logs.
type vlLogEntry struct {
	Timestamp string            `json:"timestamp"`
	Labels    map[string]string `json:"labels"`
	Level     string            `json:"level,omitempty"`
	Message   string            `json:"message"`
}

// parseVictoriaLogsResponse handles both JSON array and JSONL responses.
func parseVictoriaLogsResponse(body []byte) ([]vlLogEntry, error) {
	// Try JSON array first
	var rawEntries []struct {
		Time    string `json:"_time"`
		Stream  string `json:"_stream"`
		Level   string `json:"level"`
		Message string `json:"_msg"`
	}
	if err := json.Unmarshal(body, &rawEntries); err == nil {
		entries := make([]vlLogEntry, 0, len(rawEntries))
		for _, e := range rawEntries {
			labels := map[string]string{}
			if e.Stream != "" {
				labels["_stream"] = e.Stream
			}
			entries = append(entries, vlLogEntry{
				Timestamp: e.Time,
				Labels:    labels,
				Level:     e.Level,
				Message:   e.Message,
			})
		}
		return entries, nil
	}

	// Fallback to JSONL
	var entries []vlLogEntry
	for _, line := range splitLines(string(body)) {
		var raw map[string]any
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}

		entry := vlLogEntry{Labels: make(map[string]string)}
		if v, ok := raw["_time"].(string); ok {
			entry.Timestamp = v
		}
		if v, ok := raw["_msg"].(string); ok {
			entry.Message = v
		}
		if v, ok := raw["level"].(string); ok {
			entry.Level = v
		}
		if v, ok := raw["_stream"].(string); ok {
			entry.Labels["_stream"] = v
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

var _ Tool = (*VictoriaLogsTool)(nil)
