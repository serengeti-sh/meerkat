package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var severityRank = map[string]int{
	"info":     0,
	"warning":  1,
	"critical": 2,
}

func shouldSend(severity, minSeverity string) bool {
	if minSeverity == "" {
		return true
	}
	return severityRank[severity] >= severityRank[minSeverity]
}

// SlackChannel sends reports to Slack via webhook.
type SlackChannel struct {
	webhookURL  string
	minSeverity string
}

func NewSlackChannel(webhookURL, minSeverity string) *SlackChannel {
	return &SlackChannel{webhookURL: webhookURL, minSeverity: minSeverity}
}

func (c *SlackChannel) Type() string { return "slack" }

func (c *SlackChannel) Send(ctx context.Context, report *ReportData) error {
	if !shouldSend(report.Severity, c.minSeverity) {
		return nil
	}

	emoji := ":information_source:"
	switch report.Severity {
	case "warning":
		emoji = ":warning:"
	case "critical":
		emoji = ":rotating_light:"
	}

	payload := map[string]any{
		"blocks": []map[string]any{
			{
				"type": "header",
				"text": map[string]string{
					"type": "plain_text",
					"text": fmt.Sprintf("%s Inspector Alert [%s]", emoji, report.Severity),
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": report.Summary,
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": fmt.Sprintf("```\n%s\n```", truncate(report.Detail, 2900)),
				},
			},
			{
				"type": "context",
				"elements": []map[string]string{
					{"type": "mrkdwn", "text": fmt.Sprintf("Trigger: %s | ID: %s | Iterations: %d | %s",
						report.Trigger, report.ID, report.Iterations, report.CreatedAt.Format(time.RFC3339))},
				},
			},
		},
	}

	return doPost(ctx, c.webhookURL, payload)
}

// WebhookChannel sends reports to a custom webhook.
type WebhookChannel struct {
	url         string
	minSeverity string
}

func NewWebhookChannel(url, minSeverity string) *WebhookChannel {
	return &WebhookChannel{url: url, minSeverity: minSeverity}
}

func (c *WebhookChannel) Type() string { return "webhook" }

func (c *WebhookChannel) Send(ctx context.Context, report *ReportData) error {
	if !shouldSend(report.Severity, c.minSeverity) {
		return nil
	}

	payload := map[string]any{
		"id":          report.ID,
		"trigger":     report.Trigger,
		"trigger_id":  report.TriggerID,
		"severity":    report.Severity,
		"summary":     report.Summary,
		"detail":      report.Detail,
		"datasources": report.Datasources,
		"iterations":  report.Iterations,
		"created_at":  report.CreatedAt,
	}

	return doPost(ctx, c.url, payload)
}

func doPost(ctx context.Context, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
