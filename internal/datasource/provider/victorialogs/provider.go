package victorialogs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mandacode-labs/inspector/internal/datasource"
)

type provider struct {
	name    string
	baseURL string
	client  *http.Client
}

// New creates a new Victoria Logs provider.
func New(name, baseURL string) datasource.Provider {
	return &provider{
		name:    name,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *provider) Name() string                                      { return p.name }
func (p *provider) Type() datasource.Type                             { return datasource.TypeVictoriaLogs }
func (p *provider) MetricsQuerier() (datasource.MetricsQuerier, bool) { return nil, false }

func (p *provider) LogsQuerier() (datasource.LogsQuerier, bool) {
	return p, true
}

func (p *provider) TestConnection(ctx context.Context) error {
	u, _ := url.Parse(p.baseURL)
	u.Path = "/select/logsql/query"
	q := u.Query()
	q.Set("query", "_stream:*")
	q.Set("limit", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("connection failed: status %d", resp.StatusCode)
	}
	return nil
}

// QueryLogs executes a LogsQL query against Victoria Logs.
func (p *provider) QueryLogs(ctx context.Context, query string, limit int) ([]datasource.LogEntry, error) {
	u, _ := url.Parse(p.baseURL)
	u.Path = "/select/logsql/query"
	q := u.Query()
	q.Set("query", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	u.RawQuery = q.Encode()

	body, err := p.doGet(ctx, u.String())
	if err != nil {
		return nil, err
	}

	return parseLogsResponse(body)
}

func (p *provider) doGet(ctx context.Context, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// parseLogsResponse handles both JSON array and JSONL responses from Victoria Logs.
func parseLogsResponse(body []byte) ([]datasource.LogEntry, error) {
	// Try JSON array first
	var rawEntries []struct {
		Time    string `json:"_time"`
		Stream  string `json:"_stream"`
		Level   string `json:"level"`
		Message string `json:"_msg"`
	}
	if err := json.Unmarshal(body, &rawEntries); err == nil {
		entries := make([]datasource.LogEntry, 0, len(rawEntries))
		for _, e := range rawEntries {
			labels := map[string]string{}
			if e.Stream != "" {
				labels["_stream"] = e.Stream
			}
			entries = append(entries, datasource.LogEntry{
				Timestamp: e.Time,
				Labels:    labels,
				Level:     e.Level,
				Message:   e.Message,
			})
		}
		return entries, nil
	}

	// Fallback to JSONL
	var entries []datasource.LogEntry
	for _, line := range splitLines(string(body)) {
		var raw map[string]any
		if json.Unmarshal([]byte(line), &raw) != nil {
			continue
		}

		entry := datasource.LogEntry{
			Labels: make(map[string]string),
		}
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
