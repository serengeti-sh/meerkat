package handler

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/serengeti-sh/meerkat/internal/apperrors"

	"github.com/serengeti-sh/meerkat/internal/inspector"
)

type Handler struct {
	inspectorSvc inspector.InspectorService
}

func NewHandler(
	inspectorSvc inspector.InspectorService,
) *Handler {
	return &Handler{
		inspectorSvc: inspectorSvc,
	}
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

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func mapError(appErr apperrors.Error) int {
	return apperrors.HTTPStatus(appErr.Type())
}
