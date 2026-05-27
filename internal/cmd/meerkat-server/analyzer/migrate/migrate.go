package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/ent"
	"github.com/serengeti-sh/meerkat/internal/inspector"
)

// ApplyMigrations applies database migrations.
func ApplyMigrations(cfg *config.Config) error {
	entClient, err := inspector.NewEntClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = entClient.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := autoMigrate(ctx, entClient); err != nil {
		return err
	}

	fmt.Println("Migrations applied successfully")
	return nil
}

func autoMigrate(ctx context.Context, client *ent.Client) error {
	return inspector.Migrate(ctx, client)
}
