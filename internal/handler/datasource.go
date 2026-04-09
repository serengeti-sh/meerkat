package handler

import (
	"fmt"
	"net/http"
)

func (h *Handler) ListDatasources(w http.ResponseWriter, r *http.Request) {
	providers := h.registry.All()
	result := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		result = append(result, map[string]any{
			"name": p.Name(),
			"type": p.Type(),
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) TestDatasource(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	p, err := h.registry.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("datasource %q not found", name))
		return
	}

	if err := p.TestConnection(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("connection failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "name": name})
}
