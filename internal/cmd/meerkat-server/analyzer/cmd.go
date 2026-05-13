package analyzer

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/analyzer/migrate"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/analyzer/serve"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyzer",
		Short: "Run the Meerkat analyzer server",
	}

	cmd.AddCommand(serve.NewCmd())
	cmd.AddCommand(migrate.NewCmd())

	return cmd
}
