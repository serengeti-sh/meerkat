package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// PrometheusTool queries metrics from a single Prometheus/VictoriaMetrics endpoint.
type PrometheusTool struct {
	name        string
	description string
	params      json.RawMessage
	v1api       v1.API
}

// NewPrometheusTool creates a tool backed by one Prometheus-compatible endpoint.
func NewPrometheusTool(name, description, paramSchemaFile, baseURL string, client *http.Client) (Tool, error) {
	if name == "" {
		return nil, fmt.Errorf("prometheus tool: name is required")
	}
	if description == "" {
		return nil, fmt.Errorf("prometheus tool %q: description is required", name)
	}
	if paramSchemaFile == "" {
		return nil, fmt.Errorf("prometheus tool %q: param_schema_file is required", name)
	}
	if baseURL == "" {
		return nil, fmt.Errorf("prometheus tool %q: url is required", name)
	}

	params, err := os.ReadFile(paramSchemaFile)
	if err != nil {
		return nil, fmt.Errorf("prometheus tool %q: failed to read param schema %q: %w", name, paramSchemaFile, err)
	}

	promClient, err := api.NewClient(api.Config{
		Address:      baseURL,
		RoundTripper: client.Transport,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid prometheus address %q: %w", baseURL, err)
	}

	return &PrometheusTool{
		name:        name,
		description: description,
		params:      json.RawMessage(params),
		v1api:       v1.NewAPI(promClient),
	}, nil
}

func (t *PrometheusTool) Name() string { return t.name }

func (t *PrometheusTool) Description() string { return t.description }

func (t *PrometheusTool) Parameters() json.RawMessage { return t.params }

func (t *PrometheusTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	result, _, err := t.v1api.Query(ctx, params.Query, time.Now())
	if err != nil {
		return "", fmt.Errorf("metrics query failed: %w", err)
	}

	series, err := convertPromResult(result)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(series)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

// --- internal types ---

type timeSeries struct {
	Labels map[string]string `json:"labels"`
	Points []dataPoint       `json:"points"`
}

type dataPoint struct {
	Timestamp float64 `json:"timestamp"`
	Value     float64 `json:"value"`
}

func convertPromResult(value model.Value) ([]timeSeries, error) {
	switch value.Type() {
	case model.ValVector:
		vec := value.(model.Vector)
		series := make([]timeSeries, 0, len(vec))
		for _, sample := range vec {
			labels := make(map[string]string, len(sample.Metric))
			for k, v := range sample.Metric {
				labels[string(k)] = string(v)
			}
			series = append(series, timeSeries{
				Labels: labels,
				Points: []dataPoint{{
					Timestamp: float64(sample.Timestamp) / 1000,
					Value:     float64(sample.Value),
				}},
			})
		}
		return series, nil

	case model.ValMatrix:
		mat := value.(model.Matrix)
		series := make([]timeSeries, 0, len(mat))
		for _, stream := range mat {
			labels := make(map[string]string, len(stream.Metric))
			for k, v := range stream.Metric {
				labels[string(k)] = string(v)
			}
			points := make([]dataPoint, 0, len(stream.Values))
			for _, pair := range stream.Values {
				points = append(points, dataPoint{
					Timestamp: float64(pair.Timestamp) / 1000,
					Value:     float64(pair.Value),
				})
			}
			series = append(series, timeSeries{
				Labels: labels,
				Points: points,
			})
		}
		return series, nil

	case model.ValScalar:
		s := value.(*model.Scalar)
		return []timeSeries{{
			Points: []dataPoint{{
				Timestamp: float64(s.Timestamp) / 1000,
				Value:     float64(s.Value),
			}},
		}}, nil

	default:
		return nil, fmt.Errorf("unsupported result type: %s", value.Type())
	}
}

var _ Tool = (*PrometheusTool)(nil)
