package serve

import "github.com/spf13/cobra"

func NewCmd() *cobra.Command {
	var cfgFile string
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Vectors gRPC and OTLP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cfgFile, port)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	cmd.Flags().IntVarP(&port, "port", "p", 0, "gRPC server port (overrides config)")

	return cmd
}
