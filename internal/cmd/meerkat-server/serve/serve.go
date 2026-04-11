package serve

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/serengeti-sh/meerkat/ent"
	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/datasource"
	"github.com/serengeti-sh/meerkat/internal/datasource/provider/loki"
	"github.com/serengeti-sh/meerkat/internal/datasource/provider/prometheus"
	"github.com/serengeti-sh/meerkat/internal/datasource/provider/victorialogs"
	"github.com/serengeti-sh/meerkat/internal/handler"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	insprepo "github.com/serengeti-sh/meerkat/internal/inspector/repository"
	"github.com/serengeti-sh/meerkat/internal/reporter"
	"github.com/serengeti-sh/meerkat/internal/scheduler"
	"github.com/serengeti-sh/meerkat/internal/store"
	"go.uber.org/fx"
)

func ProvideConfig(cfgFile string, port int) (*config.Config, error) {
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

	if port != 8080 {
		cfg.HTTP.Port = port
	}

	return cfg, nil
}

func ProvideEntClient(cfg *config.Config, lc fx.Lifecycle) (*ent.Client, error) {
	client, err := store.NewEntClient(cfg)
	if err != nil {
		return nil, err
	}

	if err := store.Migrate(context.Background(), client); err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}

func ProvideAnalyzerProvider(cfg *config.Config) analyzer.LLMProvider {
	return analyzer.NewLLMProvider(analyzer.ProviderConfig{
		Provider:    cfg.Analyzer.Provider,
		URL:         cfg.Analyzer.URL,
		APIKey:      cfg.Analyzer.APIKey,
		Model:       cfg.Analyzer.Model,
		MaxTokens:   cfg.Analyzer.MaxTokens,
		Temperature: cfg.Analyzer.Temperature,
	})
}

func newProvider(ds config.DatasourceConfig) datasource.Provider {
	switch ds.Type {
	case "victoria-metrics", "prometheus":
		return prometheus.New(ds.Name, ds.URL)
	case "victoria-logs":
		return victorialogs.New(ds.Name, ds.URL)
	case "loki":
		return loki.New(ds.Name, ds.URL)
	default:
		log.Printf("[meerkat] unknown datasource type %q for %q, skipping", ds.Type, ds.Name)
		return nil
	}
}

func ProvideProviderRegistry(cfg *config.Config) *datasource.Registry {
	var providers []datasource.Provider
	for _, ds := range cfg.Datasources {
		p := newProvider(ds)
		if p != nil {
			providers = append(providers, p)
		}
	}
	return datasource.NewRegistry(providers)
}

// adapterRegistry adapts datasource.Registry to inspector.DatasourceRegistry.
type adapterRegistry struct {
	*datasource.Registry
}

func (a *adapterRegistry) All() []inspector.DatasourceRef {
	providers := a.Registry.All()
	refs := make([]inspector.DatasourceRef, 0, len(providers))
	for _, p := range providers {
		refs = append(refs, inspector.DatasourceRef{Name: p.Name(), Type: string(p.Type())})
	}
	return refs
}

func ProvideDatasourceRegistry(registry *datasource.Registry) inspector.DatasourceRegistry {
	return &adapterRegistry{Registry: registry}
}

func ProvideToolRegistry(registry *datasource.Registry) *analyzer.ToolRegistry {
	return analyzer.NewToolRegistry(
		inspector.NewQueryMetricsTool(registry),
		inspector.NewQueryLogsTool(registry),
	)
}

func ProvideAnalyzerService(provider analyzer.LLMProvider, registry *analyzer.ToolRegistry, cfg *config.Config) analyzer.AnalyzerService {
	systemPrompt := analyzer.MustLoadSystemPrompt(cfg.Analyzer.SystemPromptFile)
	skills := analyzer.MustLoadSkills(cfg.Analyzer.SkillsFile)
	prompt := analyzer.MergeSkillsIntoPrompt(systemPrompt, skills)
	return analyzer.NewService(provider, registry, cfg.Analyzer.MaxIterations, prompt)
}

func ProvideHTTPServer(
	h *handler.Handler,
	cfg *config.Config,
	lc fx.Lifecycle,
	sched scheduler.Scheduler,
) *http.Server {
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("failed to listen on %s: %w", addr, err)
			}

			log.Printf("Starting meerkat server on %s", addr)

			go func() { _ = srv.Serve(ln) }()

			if cfg.Scheduler.Enabled {
				log.Println("Starting scheduler...")
				if err := sched.Start(ctx); err != nil {
					log.Printf("Scheduler error: %v", err)
				}
			}

			return nil
		},
		OnStop: func(ctx context.Context) error {
			sched.Stop()
			log.Println("Shutting down server...")
			return srv.Shutdown(ctx)
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

// NewFXApp creates a new fx application.
func NewFXApp(cfgFile string, port int) *fx.App {
	return fx.New(
		fx.Supply(fx.Annotate(cfgFile, fx.ResultTags(`name:"cfgFile"`))),
		fx.Supply(fx.Annotate(port, fx.ResultTags(`name:"port"`))),

		fx.Provide(
			fx.Annotate(ProvideConfig, fx.ParamTags(`name:"cfgFile"`, `name:"port"`)),
			ProvideEntClient,
		),

		fx.Provide(
			ProvideProviderRegistry,
			ProvideDatasourceRegistry,
			insprepo.NewRepository,
			ProvideAnalyzerProvider,
			ProvideToolRegistry,
			ProvideAnalyzerService,
			inspector.NewService,
			reporter.NewService,
			scheduler.NewCronScheduler,
		),

		fx.Provide(
			handler.NewHandler,
			ProvideHTTPServer,
		),

		fx.Invoke(WaitForShutdown),
		fx.StartTimeout(30*1e9),
		fx.StopTimeout(30*1e9),
	)
}
