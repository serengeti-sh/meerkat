package inspect

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func NewCmd() *cobra.Command {
	var query, metricQuery, logQuery string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Trigger a manual inspection",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := meerkat.GetClient(cmd.Context())
			if c == nil {
				return fmt.Errorf("client not initialized")
			}

			resp, err := c.CreateInspect(cmd.Context(), &api.CreateInspectReq{
				Query:       optString(query),
				MetricQuery: optString(metricQuery),
				LogQuery:    optString(logQuery),
			})
			if err != nil {
				return err
			}
			return meerkat.PrintResult(resp, "json")
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "Natural language query")
	cmd.Flags().StringVar(&metricQuery, "metric-query", "", "PromQL metric query")
	cmd.Flags().StringVar(&logQuery, "log-query", "", "LogsQL log query")

	return cmd
}

func optString(s string) api.OptString {
	if s == "" {
		return api.OptString{}
	}
	return api.NewOptString(s)
}
