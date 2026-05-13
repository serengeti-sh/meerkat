package serve

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	var cfgFile string
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the meerkat server",
		Run: func(cmd *cobra.Command, args []string) {
			app := NewFXApp(cfgFile, port)
			app.Run()
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "server port")

	return cmd
}
