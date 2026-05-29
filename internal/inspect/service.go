package inspect

import (
	"context"

	"github.com/serengeti-sh/meerkat/internal/report"
)

// Service orchestrates inspections.
type Service interface {
	Start() error
	Stop()
	Inspect(ctx context.Context, req InspectRequest) (*report.Report, error)
	InspectByWebhook(ctx context.Context, payload WebhookPayload) (*report.Report, error)
	GetReport(ctx context.Context, id string) (*report.Report, error)
	ListReports(ctx context.Context, limit int) ([]*report.Report, error)
}
