package reporter

import (
	"context"
	"log"

	"github.com/mandacode-labs/inspector/internal/config"
)

type service struct {
	channels []ReportChannel
}

func NewService(cfg *config.Config) ReporterService {
	var channels []ReportChannel

	for _, ch := range cfg.Reporter.Channels {
		switch ch.Type {
		case "slack":
			channels = append(channels, NewSlackChannel(ch.WebhookURL, ch.MinSeverity))
		case "webhook":
			channels = append(channels, NewWebhookChannel(ch.URL, ch.MinSeverity))
		}
	}

	return &service{channels: channels}
}

func (s *service) Report(ctx context.Context, report *ReportData) error {
	for _, ch := range s.channels {
		if err := ch.Send(ctx, report); err != nil {
			log.Printf("[reporter] failed to send via %s: %v", ch.Type(), err)
		}
	}
	return nil
}
