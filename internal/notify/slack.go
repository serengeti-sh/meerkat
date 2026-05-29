package notify

import (
	"fmt"
	"strings"
	"time"
)

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
