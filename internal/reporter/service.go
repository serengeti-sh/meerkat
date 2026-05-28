package reporter

import (
	"context"
	"time"
)

// ReportData is the normalized report delivered to reporters.
type ReportData struct {
	ID          string    `json:"id"`
	Trigger     string    `json:"trigger"`
	TriggerID   string    `json:"trigger_id"`
	Severity    string    `json:"severity"`
	Summary     string    `json:"summary"`
	Detail      string    `json:"detail"`
	Datasources []string  `json:"datasources"`
	Iterations  int       `json:"iterations"`
	CreatedAt   time.Time `json:"created_at"`
}

// Service delivers reports to a configured webhook URL.
type Service interface {
	Report(ctx context.Context, report *ReportData) error
}
