package datasource

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List datasources",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := meerkat.GetClient(cmd.Context())
			if c == nil {
				return fmt.Errorf("client not initialized")
			}

			dss, err := c.ListDatasources(cmd.Context())
			if err != nil {
				return err
			}
			return meerkat.PrintResult(dss, "json")
		},
	}

	return cmd
}
