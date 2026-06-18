package httphandler

import (
	"context"

	"github.com/serengeti-sh/meerkat/internal/inspect"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func (h *Handler) ReceiveWebhook(ctx context.Context, req *api.ReceiveWebhookReq) (api.ReceiveWebhookRes, error) {
	report, err := h.inspectorSvc.InspectByWebhook(ctx, inspect.WebhookPayload{
		Source:  req.Source.Value,
		Alert:   req.Alert.Value,
		Message: req.Message.Value,
	})
	if err != nil {
		h.log.Error().Err(err).Msg("webhook failed")
		return &api.ErrorStatusCode{
			StatusCode: mapError(err),
			Response:   api.Error{Error: err.Error()},
		}, nil
	}

	return mapReportToResponse(report), nil
}
