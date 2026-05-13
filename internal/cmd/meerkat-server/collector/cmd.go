package collector

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/collector/serve"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collector",
		Short: "Run the Meerkat log collector server",
	}

	cmd.AddCommand(serve.NewCmd())

	return cmd
}
