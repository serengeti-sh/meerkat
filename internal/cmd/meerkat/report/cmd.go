package report

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Manage inspection reports",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newGetCmd())

	return cmd
}
