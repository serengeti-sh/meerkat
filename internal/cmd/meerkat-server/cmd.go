package server

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/migrate"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/serve"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meerkat-server",
		Short: "Meerkat AI agent server",
	}

	cmd.AddCommand(serve.NewCmd())
	cmd.AddCommand(migrate.NewCmd())

	return cmd
}
