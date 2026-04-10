package loki

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

// New creates a new Grafana Loki provider.
func New(name, baseURL string) datasource.Provider {
	return &provider{
		name:    name,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *provider) Name() string                                      { return p.name }
func (p *provider) Type() datasource.Type                             { return datasource.TypeLoki }
func (p *provider) MetricsQuerier() (datasource.MetricsQuerier, bool) { return nil, false }

func (p *provider) LogsQuerier() (datasource.LogsQuerier, bool) {
	return p, true
}

func (p *provider) TestConnection(ctx context.Context) error {
	u, _ := url.Parse(p.baseURL)
	u.Path = "/loki/api/v1/labels"

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

// QueryLogs executes a LogQL query against Loki.
func (p *provider) QueryLogs(ctx context.Context, query string, limit int) ([]datasource.LogEntry, error) {
	u, _ := url.Parse(p.baseURL)
	u.Path = "/loki/api/v1/query_range"
	q := u.Query()
	q.Set("query", query)
	q.Set("limit", fmt.Sprintf("%d", limit))
	// Default to last hour if no time range specified
	now := time.Now()
	q.Set("start", fmt.Sprintf("%d", now.Add(-time.Hour).UnixNano()))
	q.Set("end", fmt.Sprintf("%d", now.UnixNano()))
	u.RawQuery = q.Encode()

	body, err := p.doGet(ctx, u.String())
	if err != nil {
		return nil, err
	}

	return parseLokiResponse(body)
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

// parseLokiResponse parses the Loki query_range response format.
func parseLokiResponse(body []byte) ([]datasource.LogEntry, error) {
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Stream map[string]string `json:"stream"`
				Values [][]string        `json:"values"` // [ts, line] pairs
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Loki response: %w", err)
	}

	var entries []datasource.LogEntry
	for _, r := range resp.Data.Result {
		for _, v := range r.Values {
			if len(v) < 2 {
				continue
			}
			entry := datasource.LogEntry{
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
