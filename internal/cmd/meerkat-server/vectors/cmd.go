package vectors

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/vectors/serve"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vectors",
		Short: "Run the Vectors semantic indexing and search server",
	}

	cmd.AddCommand(serve.NewCmd())

	return cmd
}
