package serve

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/serengeti-sh/meerkat/internal/collector"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/logsclient"
)

const shutdownTimeout = 30 * time.Second

// Run starts the collector server with manual dependency wiring.
func Run(cfgFile string) error {
	// 1. Load config
	cfg, err := loadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	// 2. Batcher
	batcher := collector.NewBatcher(cfg)

	// 3. MeerkatLogs client (required for ingestion)
	if cfg.MeerkatLogs.Enabled && cfg.MeerkatLogs.Address != "" {
		logsCli, err := logsclient.New(cfg.MeerkatLogs.Address)
		if err != nil {
			return fmt.Errorf("create meerkatlogs client: %w", err)
		}
		defer func() {
			if err := logsCli.Close(); err != nil {
				log.Printf("failed to close meerkatlogs client: %v", err)
			}
		}()
		batcher.WithLogsClient(logsCli)
		log.Printf("[collector] connected to meerkatlogs server at %s", cfg.MeerkatLogs.Address)
	} else {
		return fmt.Errorf("meerkat_logs must be enabled with a valid address")
	}

	// 4. gRPC server
	srv := collector.NewGRPCServer(cfg, batcher)

	if err := srv.Start(); err != nil {
		return fmt.Errorf("start grpc server: %w", err)
	}
	batcher.Start()

	// Graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownCh
	log.Printf("Received signal %v, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	batcher.Stop(ctx)
	srv.Stop()

	log.Println("Collector stopped gracefully")
	return nil
}

func loadConfig(cfgFile string) (*config.Config, error) {
	if cfgFile != "" {
		return config.LoadFromPath(cfgFile)
	}
	return config.Load()
}
