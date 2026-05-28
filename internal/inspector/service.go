package inspector

import "context"

// Service orchestrates inspections.
type Service interface {
	Inspect(ctx context.Context, req InspectRequest) (*Report, error)
	InspectByWebhook(ctx context.Context, payload WebhookPayload) (*Report, error)
	GetReport(ctx context.Context, id string) (*Report, error)
	ListReports(ctx context.Context, limit int) ([]*Report, error)
	Stop()
}
