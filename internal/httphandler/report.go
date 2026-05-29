package httphandler

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

	reports, err := h.inspectorSvc.ListReports(r.Context(), limit)
	if err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}

	result := make([]reportResponse, 0, len(reports))
	for _, r := range reports {
		result = append(result, mapReport(r))
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	report, err := h.inspectorSvc.GetReport(r.Context(), id)
	if err != nil {
		writeError(w, mapError(err), err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mapReport(report))
}
