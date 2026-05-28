package meerkatserver

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/analyzer"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/collector"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/logs"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meerkat-server",
		Short: "Meerkat AI agent server",
	}

	cmd.AddCommand(analyzer.NewCmd())
	cmd.AddCommand(collector.NewCmd())
	cmd.AddCommand(logs.NewCmd())

	return cmd
}
