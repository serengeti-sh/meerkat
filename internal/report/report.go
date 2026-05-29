package report

import "time"

// Severity represents the severity level of a report.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
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
	ID          string
	Trigger     TriggerType
	TriggerID   string
	Status      Status
	Severity    Severity
	Summary     string
	Detail      string
	Query       string
	Datasources []string
	Iterations  int
	CreatedAt   time.Time
}

// Clone returns a deep copy of the report.
func (r Report) Clone() Report {
	cp := r
	if len(r.Datasources) > 0 {
		cp.Datasources = make([]string, len(r.Datasources))
		copy(cp.Datasources, r.Datasources)
	}
	return cp
}
