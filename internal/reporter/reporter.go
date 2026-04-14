package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ReportData is a decoupled report representation for the reporter package.
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

// ReporterService delivers reports to a configured webhook URL.
type ReporterService interface {
	Report(ctx context.Context, report *ReportData) error
}

var severityRank = map[string]int{
	"info":     0,
	"warning":  1,
	"critical": 2,
}

type service struct {
	webhookURL  string
	minSeverity string
}

// NewService creates a ReporterService that sends reports to the configured webhook URL.
func NewService(webhookURL, minSeverity string) ReporterService {
	return &service{
		webhookURL:  webhookURL,
		minSeverity: minSeverity,
	}
}

func (s *service) Report(ctx context.Context, report *ReportData) error {
	if s.webhookURL == "" {
		return nil
	}

	minRank := severityRank[s.minSeverity]
	if severityRank[report.Severity] < minRank {
		return nil
	}

	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	log.Printf("[reporter] sent report %s (severity=%s) to %s", report.ID, report.Severity, s.webhookURL)
	return nil
}
