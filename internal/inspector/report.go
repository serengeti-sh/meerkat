package inspector

import (
	"time"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
)

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

// TriggerType represents the source of an inspection trigger.
type TriggerType string

const (
	TriggerManual    TriggerType = "manual"
	TriggerWebhook   TriggerType = "webhook"
	TriggerScheduled TriggerType = "scheduled"
)

// Report represents the result of an inspection.
type Report struct {
	id          string
	trigger     TriggerType
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
	trigger TriggerType,
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
func (r *Report) Trigger() TriggerType  { return r.trigger }
func (r *Report) TriggerID() string     { return r.triggerID }
func (r *Report) Status() Status        { return r.status }
func (r *Report) Severity() Severity    { return r.severity }
func (r *Report) Summary() string       { return r.summary }
func (r *Report) Detail() string        { return r.detail }
func (r *Report) Query() string         { return r.query }
func (r *Report) Datasources() []string { return r.datasources }
func (r *Report) Iterations() int       { return r.iterations }
func (r *Report) CreatedAt() time.Time  { return r.createdAt }
