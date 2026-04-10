package report

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get a specific report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := meerkat.GetClient(cmd.Context())
			if c == nil {
				return fmt.Errorf("client not initialized")
			}

			resp, err := c.GetReport(cmd.Context(), api.GetReportParams{
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
