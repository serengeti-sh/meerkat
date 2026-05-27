package rag

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/cmd/meerkat-server/rag/serve"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rag",
		Short: "Run the Meerkat RAG (Retrieval-Augmented Generation) server",
	}

	cmd.AddCommand(serve.NewCmd())

	return cmd
}
