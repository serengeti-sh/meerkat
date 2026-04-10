package meerkat

import (
	"context"

	"github.com/serengeti-sh/meerkat/pkg/api"
)

type clientKey struct{}

func GetClient(ctx context.Context) *api.Client {
	if c, ok := ctx.Value(clientKey{}).(*api.Client); ok {
		return c
	}
	return nil
}

func SetClient(ctx context.Context, c *api.Client) context.Context {
	return context.WithValue(ctx, clientKey{}, c)
}
