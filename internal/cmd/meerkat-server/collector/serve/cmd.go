package serve

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the meerkat collector server",
		Run: func(cmd *cobra.Command, args []string) {
			app := NewFXApp(cfgFile)
			app.Run()
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")

	return cmd
}
