package httphandler

import (
	"context"

	"github.com/serengeti-sh/meerkat/pkg/api"
)

func (h *Handler) GetHealth(ctx context.Context) (*api.GetHealthOK, error) {
	return &api.GetHealthOK{Status: "ok"}, nil
}
