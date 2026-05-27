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

	"github.com/serengeti-sh/meerkat/internal/apperrors"
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

var _ ReporterService = (*service)(nil)

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

const defaultHTTPTimeout = 30 * time.Second

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
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return apperrors.New(apperrors.ErrInternal,
			fmt.Sprintf("webhook returned status %d", resp.StatusCode))
	}

	log.Printf("[reporter] sent report %s (severity=%s) to %s", report.ID, report.Severity, s.webhookURL)
	return nil
}

// slackBlock represents a Slack Block Kit block.
type slackBlock struct {
	Type   string      `json:"type"`
	Text   *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

// slackText represents a Slack text object.
type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// slackPayload represents a Slack message payload.
type slackPayload struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks"`
}

func buildSlackPayload(report *ReportData) slackPayload {
	emoji := severityEmoji(report.Severity)
	text := fmt.Sprintf("%s [%s] %s", emoji, report.Severity, report.Summary)

	blocks := []slackBlock{
		{
			Type: "header",
			Text: &slackText{
				Type: "plain_text",
				Text: fmt.Sprintf("%s Meerkat Alert — %s", emoji, report.Severity),
			},
		},
		{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*Summary*\n%s", report.Summary),
			},
		},
	}

	if report.Detail != "" {
		blocks = append(blocks, slackBlock{
			Type: "section",
			Text: &slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*Detail*\n%s", report.Detail),
			},
		})
	}

	var fields []slackText
	if report.Trigger != "" {
		fields = append(fields, slackText{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Trigger*\n%s", report.Trigger),
		})
	}
	if len(report.Datasources) > 0 {
		fields = append(fields, slackText{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Datasources*\n%s", strings.Join(report.Datasources, ", ")),
		})
	}
	if !report.CreatedAt.IsZero() {
		fields = append(fields, slackText{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*Time*\n<!date^%d^{date_short} {time_secs}|%s>", report.CreatedAt.Unix(), report.CreatedAt.Format(time.RFC3339)),
		})
	}
	if len(fields) > 0 {
		blocks = append(blocks, slackBlock{
			Type:   "section",
			Fields: fields,
		})
	}

	return slackPayload{
		Text:   text,
		Blocks: blocks,
	}
}

func severityEmoji(severity string) string {
	switch severity {
	case "critical":
		return ":rotating_light:"
	case "warning":
		return ":warning:"
	case "info":
		return ":information_source:"
	default:
		return ":question:"
	}
}
