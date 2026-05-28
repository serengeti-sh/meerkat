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
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/ragclient"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

const shutdownTimeout = 30 * time.Second

// Run starts the collector server with manual dependency wiring.
func Run(cfgFile string) error {
	// 1. Load config
	cfg, err := loadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Embedder
	emb := embedder.New(cfg.Embedder.APIKey, cfg.Embedder.BaseURL, cfg.Embedder.Model)

	// 3. Vector store (required as fallback even when using RAG)
	vstore, err := vectorstore.New(cfg)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}
	defer func() {
		if err := vstore.Close(); err != nil {
			log.Printf("failed to close vector store: %v", err)
		}
	}()

	// 4. Batcher
	batcher := collector.NewBatcher(cfg, emb, vstore)

	// 5. Optional RAG client for deduplicated ingestion
	if cfg.RAG.Enabled && cfg.RAG.Address != "" {
		ragCli, err := ragclient.New(cfg.RAG.Address)
		if err != nil {
			return fmt.Errorf("create rag client: %w", err)
		}
		defer func() {
			if err := ragCli.Close(); err != nil {
				log.Printf("failed to close rag client: %v", err)
			}
		}()
		batcher.WithRAGClient(ragCli)
		log.Printf("[collector] connected to RAG server at %s", cfg.RAG.Address)
	}

	// 6. gRPC server
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
