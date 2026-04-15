package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat/inspect"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat/report"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat/webhook"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

var (
	version = "dev"
	apiURL  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "meerkat",
		Short:   "CLI for Meerkat AI agent",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if apiURL == "" {
				apiURL = "http://localhost:8080"
			}

			c, err := api.NewClient(apiURL)
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}

			ctx := meerkat.SetClient(cmd.Context(), c)
			cmd.SetContext(ctx)

			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "Meerkat server URL (default: http://localhost:8080)")

	rootCmd.AddCommand(inspect.NewCmd())
	rootCmd.AddCommand(report.NewCmd())
	rootCmd.AddCommand(webhook.NewCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
