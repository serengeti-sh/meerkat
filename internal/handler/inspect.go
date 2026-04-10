package handler

import (
	"encoding/json"
	"net/http"

	"github.com/serengeti-sh/meerkat/internal/inspector"
)

func (h *Handler) Inspect(w http.ResponseWriter, r *http.Request) {
	var req inspector.InspectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	report, appErr := h.inspectorSvc.Inspect(r.Context(), req)
	if appErr != nil {
		writeError(w, mapError(appErr), appErr.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, mapReport(report))
}

func mapReport(r *inspector.Report) map[string]any {
	return map[string]any{
		"id":          r.ID(),
		"trigger":     r.Trigger(),
		"trigger_id":  r.TriggerID(),
		"status":      string(r.Status()),
		"severity":    string(r.Severity()),
		"summary":     r.Summary(),
		"detail":      r.Detail(),
		"query":       r.Query(),
		"datasources": r.Datasources(),
		"iterations":  r.Iterations(),
		"created_at":  r.CreatedAt(),
	}
}
