package serve

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the meerkat collector server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cfgFile)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")

	return cmd
}
