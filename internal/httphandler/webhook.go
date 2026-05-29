package httphandler

import (
	"encoding/json"
	"net/http"

	"github.com/serengeti-sh/meerkat/internal/inspect"
)

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	var payload inspect.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	report, err := h.inspectorSvc.InspectByWebhook(r.Context(), payload)
	if err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, mapReport(report))
}
