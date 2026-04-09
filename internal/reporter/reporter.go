package reporter

import (
	"context"
	"time"
)

// ReportData is a decoupled report representation for the reporter package.
type ReportData struct {
	ID          string
	Trigger     string
	TriggerID   string
	Severity    string
	Summary     string
	Detail      string
	Datasources []string
	Iterations  int
	CreatedAt   time.Time
}

// ReportChannel is an interface for sending reports to external systems.
type ReportChannel interface {
	Type() string
	Send(ctx context.Context, report *ReportData) error
}

// ReporterService delivers reports to configured channels.
type ReporterService interface {
	Report(ctx context.Context, report *ReportData) error
}
