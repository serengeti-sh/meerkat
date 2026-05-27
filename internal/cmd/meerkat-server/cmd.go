package server

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/analyzer"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/collector"
	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/rag"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meerkat-server",
		Short: "Meerkat AI agent server",
	}

	cmd.AddCommand(analyzer.NewCmd())
	cmd.AddCommand(collector.NewCmd())
	cmd.AddCommand(rag.NewCmd())

	return cmd
}
