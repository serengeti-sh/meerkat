package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mandacode-labs/inspector/internal/inspector"
)

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")

	var payload inspector.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if payload.Source == "" {
		payload.Source = source
	}

	report, appErr := h.inspectorSvc.InspectByWebhook(r.Context(), payload)
	if appErr != nil {
		writeError(w, mapError(appErr), appErr.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, mapReport(report))
}
