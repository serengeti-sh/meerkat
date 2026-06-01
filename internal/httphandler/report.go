package httphandler

import (
	"context"

	"github.com/serengeti-sh/meerkat/pkg/api"
)

func (h *Handler) ListReports(ctx context.Context, params api.ListReportsParams) ([]api.ReportResponse, error) {
	limit := 50
	if params.Limit.IsSet() {
		limit = params.Limit.Value
	}

	reports, err := h.inspectorSvc.ListReports(ctx, limit)
	if err != nil {
		h.log.Error().Err(err).Msg("list reports failed")
		return nil, err
	}

	result := make([]api.ReportResponse, 0, len(reports))
	for _, r := range reports {
		result = append(result, *mapReportToResponse(r))
	}
	return result, nil
}

func (h *Handler) GetReport(ctx context.Context, params api.GetReportParams) (api.GetReportRes, error) {
	report, err := h.inspectorSvc.GetReport(ctx, params.ID)
	if err != nil {
		h.log.Error().Err(err).Str("id", params.ID).Msg("get report failed")
		return &api.ErrorStatusCode{
			StatusCode: mapError(err),
			Response:   api.Error{Error: err.Error()},
		}, nil
	}

	return mapReportToResponse(report), nil
}
