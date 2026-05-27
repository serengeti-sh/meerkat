package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/serengeti-sh/meerkat/internal/inspector"
)

type reportResponse struct {
	ID          string    `json:"id"`
	Trigger     string    `json:"trigger"`
	TriggerID   string    `json:"trigger_id"`
	Status      string    `json:"status"`
	Severity    string    `json:"severity"`
	Summary     string    `json:"summary"`
	Detail      string    `json:"detail"`
	Query       string    `json:"query"`
	Datasources []string  `json:"datasources"`
	Iterations  int       `json:"iterations"`
	CreatedAt   time.Time `json:"created_at"`
}

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

func mapReport(r *inspector.Report) reportResponse {
	return reportResponse{
		ID:          r.ID(),
		Trigger:     string(r.Trigger()),
		TriggerID:   r.TriggerID(),
		Status:      string(r.Status()),
		Severity:    string(r.Severity()),
		Summary:     r.Summary(),
		Detail:      r.Detail(),
		Query:       r.Query(),
		Datasources: r.Datasources(),
		Iterations:  r.Iterations(),
		CreatedAt:   r.CreatedAt(),
	}
}
