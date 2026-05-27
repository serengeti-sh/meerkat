package analyzer

import "time"

// Severity levels for analysis results.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// AnalysisResult is the final output of the agent loop.
type AnalysisResult struct {
	Severity    Severity  `json:"severity"`
	Summary     string    `json:"summary"`
	Detail      string    `json:"detail"`
	Datasources []string  `json:"datasources_used"`
	Iterations  int       `json:"iterations"`
	RawMessages []Message `json:"-"`
	CompletedAt time.Time `json:"completed_at"`
}
