package report

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func newListCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := meerkat.GetClient(cmd.Context())
			if c == nil {
				return fmt.Errorf("client not initialized")
			}

			reports, err := c.ListReports(cmd.Context(), api.ListReportsParams{
				Limit: api.NewOptInt(limit),
			})
			if err != nil {
				return err
			}
			return meerkat.PrintResult(reports, "json")
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "l", 20, "Number of reports to return")

	return cmd
}
