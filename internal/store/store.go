package store

import (
	"context"
	"fmt"

	"github.com/serengeti-sh/meerkat/ent"
	"github.com/serengeti-sh/meerkat/ent/migrate"

	_ "github.com/lib/pq"
	"github.com/serengeti-sh/meerkat/internal/config"
)

// NewEntClient creates an ent client with PostgreSQL driver.
func NewEntClient(cfg *config.Config) (*ent.Client, error) {
	client, err := ent.Open(cfg.Store.Driver, cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return client, nil
}

// Migrate runs auto migration.
func Migrate(ctx context.Context, client *ent.Client) error {
	if err := client.Schema.Create(ctx, migrate.WithForeignKeys(false)); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}
