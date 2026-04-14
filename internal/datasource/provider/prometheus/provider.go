package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/serengeti-sh/meerkat/internal/datasource"
)

type provider struct {
	name    string
	dsType  datasource.Type
	baseURL string
	client  *http.Client
}

// New creates a new Prometheus provider.
func New(name, baseURL string, client *http.Client) datasource.Provider {
	return &provider{
		name:    name,
		dsType:  datasource.TypePrometheus,
		baseURL: baseURL,
		client:  client,
	}
}

func (p *provider) Name() string                                { return p.name }
func (p *provider) Type() datasource.Type                       { return p.dsType }
func (p *provider) LogsQuerier() (datasource.LogsQuerier, bool) { return nil, false }

func (p *provider) MetricsQuerier() (datasource.MetricsQuerier, bool) {
	return p, true
}

func (p *provider) TestConnection(ctx context.Context) error {
	u, _ := url.Parse(p.baseURL)
	u.Path = "/api/v1/query"
	q := u.Query()
	q.Set("query", "up")
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

// QueryMetrics executes an instant PromQL query.
func (p *provider) QueryMetrics(ctx context.Context, query string) ([]datasource.TimeSeries, error) {
	u, _ := url.Parse(p.baseURL)
	u.Path = "/api/v1/query"
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	body, err := p.doGet(ctx, u.String())
	if err != nil {
		return nil, err
	}

	return parseInstantQueryResponse(body)
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

// parseInstantQueryResponse parses the standard Prometheus/VM instant query response.
func parseInstantQueryResponse(body []byte) ([]datasource.TimeSeries, error) {
	var resp struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []json.Number     `json:"value"`  // [timestamp, value] for vector
				Values [][]json.Number   `json:"values"` // [[ts, val], ...] for matrix
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var series []datasource.TimeSeries
	for _, r := range resp.Data.Result {
		var points []datasource.DataPoint

		// Matrix result: [[ts, val], [ts, val], ...]
		for _, pair := range r.Values {
			if len(pair) >= 2 {
				ts, _ := pair[0].Float64()
				val, _ := pair[1].Float64()
				points = append(points, datasource.DataPoint{Timestamp: ts, Value: val})
			}
		}

		// Vector result: [ts, val]
		if len(r.Value) >= 2 && len(points) == 0 {
			ts, _ := r.Value[0].Float64()
			val, _ := r.Value[1].Float64()
			points = append(points, datasource.DataPoint{Timestamp: ts, Value: val})
		}

		series = append(series, datasource.TimeSeries{
			Labels: r.Metric,
			Points: points,
		})
	}

	return series, nil
}
