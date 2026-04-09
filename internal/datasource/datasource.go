package datasource

import (
	"context"
	"encoding/json"
	"time"

	apperrors "github.com/mandacode-labs/inspector/internal/errors"
)

type DatasourceID string

type Type string

const (
	TypePrometheus   Type = "prometheus"
	TypeVictoriaLogs Type = "victoria-logs"
	TypeLoki         Type = "loki"
)

// --- Response types ---

// TimeSeries represents a single time series with labels and data points.
type TimeSeries struct {
	Labels map[string]string `json:"labels"`
	Points []DataPoint       `json:"points"`
}

// DataPoint represents a single metric data point.
type DataPoint struct {
	Timestamp float64 `json:"timestamp"`
	Value     float64 `json:"value"`
}

// LogEntry represents a single log line.
type LogEntry struct {
	Timestamp string            `json:"timestamp"`
	Labels    map[string]string `json:"labels"`
	Message   string            `json:"message"`
	Level     string            `json:"level,omitempty"`
}

// --- Domain interfaces ---

// MetricsQuerier queries metrics from a datasource.
type MetricsQuerier interface {
	QueryMetrics(ctx context.Context, query string) ([]TimeSeries, error)
}

// LogsQuerier queries logs from a datasource.
type LogsQuerier interface {
	QueryLogs(ctx context.Context, query string, limit int) ([]LogEntry, error)
}

// Provider represents a single datasource with optional metrics/logs support.
type Provider interface {
	Name() string
	Type() Type
	MetricsQuerier() (MetricsQuerier, bool)
	LogsQuerier() (LogsQuerier, bool)
	TestConnection(ctx context.Context) error
}

// --- Existing domain model (for DatasourceService) ---

type Datasource struct {
	id        DatasourceID
	name      string
	dsType    Type
	url       string
	extra     json.RawMessage
	createdAt time.Time
	updatedAt time.Time
}

func NewDatasource(
	id DatasourceID,
	name string,
	dsType Type,
	url string,
	extra json.RawMessage,
	createdAt time.Time,
	updatedAt time.Time,
) (*Datasource, error) {
	return &Datasource{
		id:        id,
		name:      name,
		dsType:    dsType,
		url:       url,
		extra:     extra,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}, nil
}

func (d *Datasource) ID() DatasourceID       { return d.id }
func (d *Datasource) Name() string           { return d.name }
func (d *Datasource) Type() Type             { return d.dsType }
func (d *Datasource) URL() string            { return d.url }
func (d *Datasource) Extra() json.RawMessage { return d.extra }
func (d *Datasource) CreatedAt() time.Time   { return d.createdAt }
func (d *Datasource) UpdatedAt() time.Time   { return d.updatedAt }

// DatasourceService provides read-only datasource operations from config.
type DatasourceService interface {
	List(ctx context.Context) ([]*Datasource, apperrors.Error)
	TestConnection(ctx context.Context, name string) apperrors.Error
}
