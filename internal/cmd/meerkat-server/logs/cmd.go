package logs

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/logs/serve"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Run the MeerkatLogs ingestion and search server",
	}

	cmd.AddCommand(serve.NewCmd())

	return cmd
}
