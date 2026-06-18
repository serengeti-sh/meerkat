package server

import (
	"fmt"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/vectorsclient"
)

func newVectorsClient(cfg *config.Config) (vectorsclient.Client, error) {
	if !cfg.Vectors.Enabled || cfg.Vectors.Address == "" {
		return nil, nil
	}
	client, err := vectorsclient.New(cfg.Vectors.Address)
	if err != nil {
		return nil, fmt.Errorf("create vectors client: %w", err)
	}
	return client, nil
}
