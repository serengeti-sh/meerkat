package migrate

import (
	"github.com/spf13/cobra"

	"github.com/serengeti-sh/meerkat/internal/config"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Database migration commands",
	}

	cmd.AddCommand(newApplyCmd())

	return cmd
}

func newApplyCmd() *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			var cfg *config.Config
			var err error

			if cfgFile != "" {
				cfg, err = config.LoadFromPath(cfgFile)
			} else {
				cfg, err = config.Load()
			}
			if err != nil {
				return err
			}

			return ApplyMigrations(cfg)
		},
	}

	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "config file path")

	return cmd
}
