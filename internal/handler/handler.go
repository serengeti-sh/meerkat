package handler

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/serengeti-sh/meerkat/internal/apperrors"

	"github.com/serengeti-sh/meerkat/internal/inspector"
)

type Handler struct {
	inspectorSvc inspector.Service
}

func NewHandler(
	inspectorSvc inspector.Service,
) *Handler {
	if inspectorSvc == nil {
		panic("handler: inspectorSvc is required")
	}
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

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func mapError(appErr apperrors.Error) int {
	return apperrors.HTTPStatus(appErr.Type())
}
