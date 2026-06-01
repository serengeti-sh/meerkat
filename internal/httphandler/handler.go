package httphandler

import (
	"context"
	"errors"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/serengeti-sh/meerkat/internal/errs"
	"github.com/serengeti-sh/meerkat/internal/inspect"
	"github.com/serengeti-sh/meerkat/internal/report"
)

// Inspector is the subset of inspect.Service that Handler requires.
// Defined locally so Handler depends only on what it uses.
type Inspector interface {
	Inspect(ctx context.Context, req inspect.Request) (*report.Report, error)
	InspectByWebhook(ctx context.Context, payload inspect.WebhookPayload) (*report.Report, error)
	GetReport(ctx context.Context, id string) (*report.Report, error)
	ListReports(ctx context.Context, limit int) ([]*report.Report, error)
}

type Handler struct {
	inspectorSvc Inspector
	log          zerolog.Logger
}

func New(inspectorSvc Inspector, log zerolog.Logger) *Handler {
	if inspectorSvc == nil {
		panic("handler: inspectorSvc is required")
	}
	return &Handler{
		inspectorSvc: inspectorSvc,
		log:          log,
	}
}

func mapError(err error) int {
	var appErr errs.AppError
	if errors.As(err, &appErr) {
		return errs.HTTPStatus(appErr.Type())
	}
	return http.StatusInternalServerError
}
