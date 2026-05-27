package handler

import (
	"net/http"
	"strconv"
)

const defaultListLimit = 50

func (h *Handler) ListReports(w http.ResponseWriter, r *http.Request) {
	limit := defaultListLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	reports, appErr := h.inspectorSvc.ListReports(r.Context(), limit)
	if appErr != nil {
		writeError(w, mapError(appErr), appErr.Error())
		return
	}

	result := make([]map[string]any, 0, len(reports))
	for _, r := range reports {
		result = append(result, mapReport(r))
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	report, appErr := h.inspectorSvc.GetReport(r.Context(), id)
	if appErr != nil {
		writeError(w, mapError(appErr), appErr.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapReport(report))
}
