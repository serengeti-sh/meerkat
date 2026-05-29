package serve

import (
	"fmt"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/server"
)

const defaultHTTPPort = 8080

// Run starts the analyzer server.
func Run(cfgFile string, port int) error {
	cfg, err := loadConfig(cfgFile, port)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	app, err := server.NewAnalyzer(cfg)
	if err != nil {
		return fmt.Errorf("create analyzer: %w", err)
	}

	return app.Run()
}

func loadConfig(cfgFile string, port int) (*config.Config, error) {
	var cfg *config.Config
	var err error

	if cfgFile != "" {
		cfg, err = config.LoadFromPath(cfgFile)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		return nil, err
	}

	if port != defaultHTTPPort {
		cfg.HTTP.Port = port
	}

	return cfg, nil
}
