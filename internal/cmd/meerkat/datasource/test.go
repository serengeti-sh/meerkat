package datasource

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <name>",
		Short: "Test datasource connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := meerkat.GetClient(cmd.Context())
			if c == nil {
				return fmt.Errorf("client not initialized")
			}

			resp, err := c.TestDatasource(cmd.Context(), api.TestDatasourceParams{
				ID: args[0],
			})
			if err != nil {
				return err
			}
			return meerkat.PrintResult(resp, "json")
		},
	}

	return cmd
}
