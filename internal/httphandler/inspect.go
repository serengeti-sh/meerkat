package httphandler

import (
	"context"

	"github.com/serengeti-sh/meerkat/internal/inspect"
	"github.com/serengeti-sh/meerkat/internal/report"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func (h *Handler) CreateInspect(ctx context.Context, req *api.CreateInspectReq) (api.CreateInspectRes, error) {
	report, err := h.inspectorSvc.Inspect(ctx, inspect.Request{
		Query:       req.Query.Value,
		MetricQuery: req.MetricQuery.Value,
		LogQuery:    req.LogQuery.Value,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("inspect failed")
		return &api.ErrorStatusCode{
			StatusCode: mapError(err),
			Response:   api.Error{Error: err.Error()},
		}, nil
	}

	return mapReportToResponse(report), nil
}

func mapReportToResponse(r *report.Report) *api.ReportResponse {
	return &api.ReportResponse{
		ID:        r.ID,
		Trigger:   api.ReportResponseTrigger(r.Trigger),
		TriggerID: r.TriggerID,
		Status:    api.ReportResponseStatus(r.Status),
		Severity:  api.NewOptReportResponseSeverity(api.ReportResponseSeverity(r.Severity)),
		Summary:   api.NewOptString(r.Summary),
		Detail:    api.NewOptString(r.Detail),
		Query:     api.NewOptString(r.Query),
		Datasources: r.Datasources,
		Iterations:  api.NewOptInt(r.Iterations),
		CreatedAt:   api.NewOptDateTime(r.CreatedAt),
	}
}
