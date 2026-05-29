package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/serengeti-sh/meerkat/internal/errs"
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

func severityRank(s string) int {
	switch s {
	case "warning":
		return 1
	case "critical":
		return 2
	default:
		return 0
	}
}

type service struct {
	webhookURL  string
	minSeverity string
	httpClient  *http.Client
}

var _ Service = (*service)(nil)

const defaultHTTPTimeout = 30 * time.Second

// NewService creates a Service that sends reports to the configured webhook URL.
func NewService(webhookURL, minSeverity string, httpClient *http.Client) *service {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &service{
		webhookURL:  webhookURL,
		minSeverity: minSeverity,
		httpClient:  httpClient,
	}
}

func (s *service) Report(ctx context.Context, report *ReportData) error {
	if s.webhookURL == "" {
		return nil // no webhook configured
	}

	minRank := severityRank(s.minSeverity)
	if severityRank(report.Severity) < minRank {
		return nil // severity below threshold
	}

	payload := buildSlackPayload(report)
	body, err := json.Marshal(payload)
	if err != nil {
		return errs.Wrap(errs.ErrInternal, "marshal slack payload", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return errs.Wrap(errs.ErrInternal, "create webhook request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Sanitize: the raw error may contain the secret webhook URL.
		return errs.Wrap(errs.ErrInternal, "send webhook request failed", fmt.Errorf("network error"))
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return errs.New(errs.ErrInternal,
			fmt.Sprintf("webhook returned status %d", resp.StatusCode))
	}

	log.Printf("[notify] sent report %s (severity=%s) to %s", report.ID, report.Severity, s.webhookURL)
	return nil
}
