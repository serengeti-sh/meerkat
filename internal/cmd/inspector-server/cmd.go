package server

import (
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspector-server",
		Short: "Inspector AI agent server",
	}

	cmd.AddCommand(newServeCmd())
	cmd.AddCommand(newMigrateCmd())

	return cmd
}

func newServeCmd() *cobra.Command {
	var cfgFile string
	var port int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the inspector server",
		Run: func(cmd *cobra.Command, args []string) {
			app := NewFXApp(cfgFile, port)
			app.Run()
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "server port")

	return cmd
}

func newMigrateCmd() *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "apply",
		Short: "Apply database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ApplyMigrations(cfgFile)
		},
	})

	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file path")

	return cmd
}
