package inspector

import (
	"context"
	"encoding/json"
	"time"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	apperrors "github.com/serengeti-sh/meerkat/internal/apperrors"
)

// InspectorService orchestrates inspections.
type InspectorService interface {
	Inspect(ctx context.Context, req InspectRequest) (*Report, apperrors.Error)
	InspectByWebhook(ctx context.Context, payload WebhookPayload) (*Report, apperrors.Error)
	GetReport(ctx context.Context, id string) (*Report, apperrors.Error)
	ListReports(ctx context.Context, limit int) ([]*Report, apperrors.Error)
	Stop()
}

// Severity type for reports.
type Severity = analyzer.Severity

const (
	SeverityInfo     = analyzer.SeverityInfo
	SeverityWarning  = analyzer.SeverityWarning
	SeverityCritical = analyzer.SeverityCritical
)

// Status represents the report lifecycle.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

// Report represents the result of an inspection.
type Report struct {
	id          string
	trigger     string // manual, webhook, scheduled
	triggerID   string
	status      Status
	severity    Severity
	summary     string
	detail      string
	query       string // original request query for dedup
	datasources []string
	iterations  int
	createdAt   time.Time
}

func NewReport(
	id string,
	trigger string,
	triggerID string,
	status Status,
	severity Severity,
	summary string,
	detail string,
	query string,
	datasources []string,
	iterations int,
	createdAt time.Time,
) *Report {
	return &Report{
		id:          id,
		trigger:     trigger,
		triggerID:   triggerID,
		status:      status,
		severity:    severity,
		summary:     summary,
		detail:      detail,
		query:       query,
		datasources: datasources,
		iterations:  iterations,
		createdAt:   createdAt,
	}
}

func (r *Report) ID() string            { return r.id }
func (r *Report) Trigger() string       { return r.trigger }
func (r *Report) TriggerID() string     { return r.triggerID }
func (r *Report) Status() Status        { return r.status }
func (r *Report) Severity() Severity    { return r.severity }
func (r *Report) Summary() string       { return r.summary }
func (r *Report) Detail() string        { return r.detail }
func (r *Report) Query() string         { return r.query }
func (r *Report) Datasources() []string { return r.datasources }
func (r *Report) Iterations() int       { return r.iterations }
func (r *Report) CreatedAt() time.Time  { return r.createdAt }

// InspectRequest is the input for a manual inspection.
type InspectRequest struct {
	MetricQuery string `json:"metric_query,omitempty"`
	LogQuery    string `json:"log_query,omitempty"`
	Query       string `json:"query,omitempty"` // natural language query
}

// WebhookPayload is the input for a webhook-triggered inspection.
type WebhookPayload struct {
	Source  string          `json:"source"` // grafana, custom
	Alert   string          `json:"alert"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// ReportRepository stores and retrieves reports.
type ReportRepository interface {
	Create(ctx context.Context, report *Report) error
	Update(ctx context.Context, report *Report) error
	GetByID(ctx context.Context, id string) (*Report, error)
	List(ctx context.Context, limit int) ([]*Report, error)
	FindActiveByQuery(ctx context.Context, trigger string, query string, since time.Time) (*Report, error)
}
