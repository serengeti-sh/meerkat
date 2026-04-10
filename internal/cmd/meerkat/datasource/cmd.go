package datasource

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datasource",
		Short: "Manage datasources",
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newTestCmd())

	return cmd
}
