package inspector

import (
	"context"

	apperrors "github.com/serengeti-sh/meerkat/internal/apperrors"
)

// Service orchestrates inspections.
type Service interface {
	Inspect(ctx context.Context, req InspectRequest) (*Report, apperrors.Error)
	InspectByWebhook(ctx context.Context, payload WebhookPayload) (*Report, apperrors.Error)
	GetReport(ctx context.Context, id string) (*Report, apperrors.Error)
	ListReports(ctx context.Context, limit int) ([]*Report, apperrors.Error)
	Stop()
}
