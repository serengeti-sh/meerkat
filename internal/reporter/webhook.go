package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

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
	httpClient  *http.Client
}

// NewService creates a ReporterService that sends reports to the configured webhook URL.
func NewService(webhookURL, minSeverity string, httpClient *http.Client) ReporterService {
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
		return nil
	}

	minRank := severityRank[s.minSeverity]
	if severityRank[report.Severity] < minRank {
		return nil
	}

	payload, err := json.Marshal(buildSlackPayload(report))
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
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

func buildSlackPayload(report *ReportData) map[string]any {
	emoji := severityEmoji(report.Severity)
	text := fmt.Sprintf("%s [%s] %s", emoji, report.Severity, report.Summary)

	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{
				"type": "plain_text",
				"text": fmt.Sprintf("%s Meerkat Alert — %s", emoji, report.Severity),
			},
		},
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Summary*\n%s", report.Summary),
			},
		},
	}

	if report.Detail != "" {
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*Detail*\n%s", report.Detail),
			},
		})
	}

	fields := []map[string]any{}
	if report.Trigger != "" {
		fields = append(fields,
			map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Trigger*\n%s", report.Trigger)},
		)
	}
	if len(report.Datasources) > 0 {
		fields = append(fields,
			map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Datasources*\n%s", strings.Join(report.Datasources, ", "))},
		)
	}
	if !report.CreatedAt.IsZero() {
		fields = append(fields,
			map[string]any{"type": "mrkdwn", "text": fmt.Sprintf("*Time*\n<!date^%d^{date_short} {time_secs}|%s>", report.CreatedAt.Unix(), report.CreatedAt.Format(time.RFC3339))},
		)
	}
	if len(fields) > 0 {
		blocks = append(blocks, map[string]any{
			"type":   "section",
			"fields": fields,
		})
	}

	return map[string]any{
		"text":   text,
		"blocks": blocks,
	}
}

func severityEmoji(severity string) string {
	switch severity {
	case "critical":
		return ":rotating_light:"
	case "warning":
		return ":warning:"
	default:
		return ":information_source:"
	}
}
