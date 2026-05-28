package report

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

// ReportOption configures a Report via the builder.
type ReportOption func(*Report)

// WithID sets the report ID.
func WithID(id string) ReportOption {
	return func(r *Report) { r.id = id }
}

// WithTrigger sets the trigger type.
func WithTrigger(t TriggerType) ReportOption {
	return func(r *Report) { r.trigger = t }
}

// WithTriggerID sets the trigger ID.
func WithTriggerID(id string) ReportOption {
	return func(r *Report) { r.triggerID = id }
}

// WithStatus sets the status.
func WithStatus(s Status) ReportOption {
	return func(r *Report) { r.status = s }
}

// WithSeverity sets the severity.
func WithSeverity(s Severity) ReportOption {
	return func(r *Report) { r.severity = s }
}

// WithSummary sets the summary.
func WithSummary(s string) ReportOption {
	return func(r *Report) { r.summary = s }
}

// WithDetail sets the detail.
func WithDetail(d string) ReportOption {
	return func(r *Report) { r.detail = d }
}

// WithQuery sets the original query.
func WithQuery(q string) ReportOption {
	return func(r *Report) { r.query = q }
}

// WithDatasources sets the datasource names.
func WithDatasources(ds []string) ReportOption {
	return func(r *Report) { r.datasources = ds }
}

// WithIterations sets the iteration count.
func WithIterations(n int) ReportOption {
	return func(r *Report) { r.iterations = n }
}

// WithCreatedAt sets the creation time.
func WithCreatedAt(t time.Time) ReportOption {
	return func(r *Report) { r.createdAt = t }
}

// NewReport creates a Report with the given options.
func NewReport(opts ...ReportOption) *Report {
	r := &Report{}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Clone returns a deep copy of the report with any options applied on top.
// This is the preferred way to create a modified copy (e.g., change status).
func (r *Report) Clone(opts ...ReportOption) *Report {
	cp := &Report{
		id:          r.id,
		trigger:     r.trigger,
		triggerID:   r.triggerID,
		status:      r.status,
		severity:    r.severity,
		summary:     r.summary,
		detail:      r.detail,
		query:       r.query,
		datasources: append([]string(nil), r.datasources...),
		iterations:  r.iterations,
		createdAt:   r.createdAt,
	}
	for _, opt := range opts {
		opt(cp)
	}
	return cp
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
