package serve

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/serengeti-sh/meerkat/internal/collector"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
	"go.uber.org/fx"
)

func ProvideConfig(cfgFile string) (*config.Config, error) {
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

	return cfg, nil
}

func ProvideEmbedder(cfg *config.Config) embedder.Embedder {
	return embedder.New(cfg.Embedder.APIKey, cfg.Embedder.BaseURL, cfg.Embedder.Model)
}

func ProvideVectorStore(cfg *config.Config, lc fx.Lifecycle) (vectorstore.VectorStore, error) {
	vs, err := vectorstore.NewMilvusClient(cfg)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return vs.Close()
		},
	})

	return vs, nil
}

func ProvideBatcher(cfg *config.Config, emb embedder.Embedder, vs vectorstore.VectorStore) *collector.Batcher {
	return collector.NewBatcher(cfg, emb, vs)
}

func ProvideGRPCServer(cfg *config.Config, batcher *collector.Batcher, lc fx.Lifecycle) *collector.GRPCServer {
	srv := collector.NewGRPCServer(cfg, batcher)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := srv.Start(); err != nil {
				return fmt.Errorf("start grpc server: %w", err)
			}
			batcher.Start()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			batcher.Stop(ctx)
			srv.Stop()
			return nil
		},
	})

	return srv
}

func WaitForShutdown(lc fx.Lifecycle) {
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				sig := <-shutdownCh
				log.Printf("Received signal %v, shutting down...", sig)
			}()
			return nil
		},
	})
}

// NewFXApp creates a new fx application for the collector.
func NewFXApp(cfgFile string) *fx.App {
	return fx.New(
		fx.Supply(fx.Annotate(cfgFile, fx.ResultTags(`name:"cfgFile"`))),

		fx.Provide(
			fx.Annotate(ProvideConfig, fx.ParamTags(`name:"cfgFile"`)),
		),

		fx.Provide(
			ProvideEmbedder,
			ProvideVectorStore,
			ProvideBatcher,
			ProvideGRPCServer,
		),

		fx.Invoke(WaitForShutdown),
		fx.StartTimeout(30*1e9),
		fx.StopTimeout(30*1e9),
	)
}
