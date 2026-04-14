package handler

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/serengeti-sh/meerkat/internal/errors"

	"github.com/serengeti-sh/meerkat/internal/datasource"
	"github.com/serengeti-sh/meerkat/internal/inspector"
)

type Handler struct {
	inspectorSvc inspector.InspectorService
	registry     *datasource.Registry
}

func NewHandler(
	inspectorSvc inspector.InspectorService,
	registry *datasource.Registry,
) *Handler {
	return &Handler{
		inspectorSvc: inspectorSvc,
		registry:     registry,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/health", h.Health)
	mux.HandleFunc("POST /v1/inspect", h.Inspect)
	mux.HandleFunc("POST /v1/webhook", h.Webhook)
	mux.HandleFunc("GET /v1/reports", h.ListReports)
	mux.HandleFunc("GET /v1/reports/{id}", h.GetReport)
	mux.HandleFunc("GET /v1/datasources", h.ListDatasources)
	mux.HandleFunc("GET /v1/datasources/{name}/test", h.TestDatasource)
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
