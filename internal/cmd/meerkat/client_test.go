package meerkat_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	meerkat "github.com/serengeti-sh/meerkat/internal/cmd/meerkat"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func TestGetClient(t *testing.T) {
	t.Run("client exists", func(t *testing.T) {
		client, _ := api.NewClient("http://localhost:8080")
		ctx := meerkat.SetClient(context.Background(), client)
		fetched := meerkat.GetClient(ctx)
		assert.NotNil(t, fetched)
	})

	t.Run("client missing", func(t *testing.T) {
		ctx := context.Background()
		fetched := meerkat.GetClient(ctx)
		assert.Nil(t, fetched)
	})
}

func TestSetClient(t *testing.T) {
	client, _ := api.NewClient("http://localhost:8080")
	ctx := meerkat.SetClient(context.Background(), client)
	assert.NotNil(t, ctx)

	// Verify the client is stored
	fetched := meerkat.GetClient(ctx)
	assert.Equal(t, client, fetched)
}
