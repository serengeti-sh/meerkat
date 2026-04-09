package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mandacode-labs/inspector/pkg/api"
)

var (
	version = "dev"
	apiURL  string
	client  *api.Client
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "inspector",
		Short:   "CLI client for Inspector AI agent",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if apiURL == "" {
				apiURL = "http://localhost:8080"
			}
			c, err := api.NewClient(apiURL)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
			client = c
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Inspector server URL (default: http://localhost:8080)")

	rootCmd.AddCommand(newInspectCmd())
	rootCmd.AddCommand(newReportCmd())
	rootCmd.AddCommand(newDatasourceCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newInspectCmd() *cobra.Command {
	var query, metricQuery, logQuery string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Trigger a manual inspection",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client.CreateInspect(cmd.Context(), &api.CreateInspectReq{
				Query:       optString(query),
				MetricQuery: optString(metricQuery),
				LogQuery:    optString(logQuery),
			})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	cmd.Flags().StringVarP(&query, "query", "q", "", "Natural language query")
	cmd.Flags().StringVar(&metricQuery, "metric-query", "", "PromQL metric query")
	cmd.Flags().StringVar(&logQuery, "log-query", "", "LogsQL log query")

	return cmd
}

func newReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Manage inspection reports",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List reports",
		RunE: func(cmd *cobra.Command, args []string) error {
			reports, err := client.ListReports(cmd.Context(), api.ListReportsParams{
				Limit: api.NewOptInt(20),
			})
			if err != nil {
				return err
			}
			return printJSON(reports)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "get [id]",
		Short: "Get a specific report",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client.GetReport(cmd.Context(), api.GetReportParams{
				ID: args[0],
			})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	})

	return cmd
}

func newDatasourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datasource",
		Short: "Manage datasources",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List datasources",
		RunE: func(cmd *cobra.Command, args []string) error {
			dss, err := client.ListDatasources(cmd.Context())
			if err != nil {
				return err
			}
			return printJSON(dss)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "test [id]",
		Short: "Test datasource connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client.TestDatasource(cmd.Context(), api.TestDatasourceParams{
				ID: args[0],
			})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	})

	return cmd
}

func optString(s string) api.OptString {
	if s == "" {
		return api.OptString{}
	}
	return api.NewOptString(s)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
