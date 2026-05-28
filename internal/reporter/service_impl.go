package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/serengeti-sh/meerkat/internal/apperrors"
)

var severityRank = map[string]int{
	"info":     0,
	"warning":  1,
	"critical": 2,
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

	minRank := severityRank[s.minSeverity]
	if severityRank[report.Severity] < minRank {
		return nil // severity below threshold
	}

	payload := buildSlackPayload(report)
	body, err := json.Marshal(payload)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternal, "marshal slack payload", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternal, "create webhook request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternal, "send webhook request", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return apperrors.New(apperrors.ErrInternal,
			fmt.Sprintf("webhook returned status %d", resp.StatusCode))
	}

	log.Printf("[reporter] sent report %s (severity=%s) to %s", report.ID, report.Severity, s.webhookURL)
	return nil
}
