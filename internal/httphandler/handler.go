package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	apperrors "github.com/serengeti-sh/meerkat/internal/apperrors"

	"github.com/serengeti-sh/meerkat/internal/inspector"
	"github.com/serengeti-sh/meerkat/internal/report"
)

// Inspector is the subset of inspector.Service that Handler requires.
// Defined locally so Handler depends only on what it uses.
type Inspector interface {
	Inspect(ctx context.Context, req inspector.InspectRequest) (*report.Report, error)
	InspectByWebhook(ctx context.Context, payload inspector.WebhookPayload) (*report.Report, error)
	GetReport(ctx context.Context, id string) (*report.Report, error)
	ListReports(ctx context.Context, limit int) ([]*report.Report, error)
}

type Handler struct {
	inspectorSvc Inspector
}

func New(
	inspectorSvc Inspector,
) (*Handler, error) {
	if inspectorSvc == nil {
		return nil, fmt.Errorf("handler: inspectorSvc is required")
	}
	return &Handler{
		inspectorSvc: inspectorSvc,
	}, nil
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/health", h.Health)
	mux.HandleFunc("POST /v1/inspect", h.Inspect)
	mux.HandleFunc("POST /v1/webhook", h.Webhook)
	mux.HandleFunc("GET /v1/reports", h.ListReports)
	mux.HandleFunc("GET /v1/reports/{id}", h.GetReport)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func mapError(err error) int {
	var appErr apperrors.Error
	if errors.As(err, &appErr) {
		return apperrors.HTTPStatus(appErr.Type())
	}
	return http.StatusInternalServerError
}
