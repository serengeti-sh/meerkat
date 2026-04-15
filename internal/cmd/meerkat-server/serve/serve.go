package serve

import (
	"context"
	"crypto/tls"
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
	"github.com/serengeti-sh/meerkat/internal/handler"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	insprepo "github.com/serengeti-sh/meerkat/internal/inspector/repository"
	"github.com/serengeti-sh/meerkat/internal/reporter"
	"github.com/serengeti-sh/meerkat/internal/scheduler"
	"github.com/serengeti-sh/meerkat/internal/store"
	"github.com/serengeti-sh/meerkat/internal/tool"
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
		Retry: analyzer.RetryConfig{
			MaxRetries: cfg.Analyzer.MaxRetries,
			BaseDelay:  time.Duration(cfg.Analyzer.RetryBaseMs) * time.Millisecond,
		},
	})
}

func ProvideToolRegistry(cfg *config.Config) (*analyzer.ToolRegistry, error) {
	var tools []tool.Tool

	for _, pc := range cfg.Tools.Prometheus {
		httpClient, err := tool.NewHTTPClient(pc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", pc.Name, err)
		}
		t, err := tool.NewPrometheusTool(pc.Name, cfg.Tools.PrometheusDescription, cfg.Tools.PrometheusParamSchemaFile, pc.URL, httpClient)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", pc.Name, err)
		}
		tools = append(tools, t)
	}

	for _, lc := range cfg.Tools.Loki {
		httpClient, err := tool.NewHTTPClient(lc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", lc.Name, err)
		}
		t, err := tool.NewLokiTool(lc.Name, cfg.Tools.LokiDescription, cfg.Tools.LokiParamSchemaFile, lc.URL, httpClient)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", lc.Name, err)
		}
		tools = append(tools, t)
	}

	for _, vc := range cfg.Tools.VictoriaLogs {
		httpClient, err := tool.NewHTTPClient(vc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", vc.Name, err)
		}
		t, err := tool.NewVictoriaLogsTool(vc.Name, cfg.Tools.VictoriaLogsDescription, cfg.Tools.VictoriaLogsParamSchemaFile, vc.URL, httpClient)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", vc.Name, err)
		}
		tools = append(tools, t)
	}

	for _, cc := range cfg.Tools.Custom {
		httpClient, err := tool.NewHTTPClient(cc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", cc.Name, err)
		}
		t, err := tool.NewCustomTool(cc.Name, cc.Description, cc.Method, cc.URL, cc.ParamSchemaFile, httpClient)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", cc.Name, err)
		}
		tools = append(tools, t)
	}

	return analyzer.NewToolRegistry(tools...), nil
}

func ProvideDatasourceRefs(cfg *config.Config) inspector.DatasourceRefs {
	return func() []analyzer.DatasourceRef {
		var refs []analyzer.DatasourceRef
		for _, pc := range cfg.Tools.Prometheus {
			refs = append(refs, analyzer.DatasourceRef{Name: pc.Name, Type: "prometheus"})
		}
		for _, lc := range cfg.Tools.Loki {
			refs = append(refs, analyzer.DatasourceRef{Name: lc.Name, Type: "loki"})
		}
		for _, vc := range cfg.Tools.VictoriaLogs {
			refs = append(refs, analyzer.DatasourceRef{Name: vc.Name, Type: "victoria-logs"})
		}
		for _, cc := range cfg.Tools.Custom {
			refs = append(refs, analyzer.DatasourceRef{Name: cc.Name, Type: "custom"})
		}
		return refs
	}
}

func ProvideDedupWindow(cfg *config.Config) time.Duration {
	return cfg.Inspector.GetDedupWindow()
}

func ProvideReporterService(cfg *config.Config) reporter.ReporterService {
	return reporter.NewService(cfg.Reporter.WebhookURL, cfg.Reporter.MinSeverity)
}

func ProvideAnalyzerService(provider analyzer.LLMProvider, registry *analyzer.ToolRegistry, cfg *config.Config) analyzer.AnalyzerService {
	systemPrompt := analyzer.MustLoadSystemPrompt(cfg.Analyzer.SystemPromptFile)
	skills := analyzer.MustLoadSkills(cfg.Analyzer.SkillsFile)
	prompt := analyzer.MergeSkillsIntoPrompt(systemPrompt, skills)
	return analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations:       cfg.Analyzer.MaxIterations,
		SystemPrompt:        prompt,
		MaxToolResultChars:  cfg.Analyzer.MaxToolResultChars,
		SummarizeOnOverflow: cfg.Analyzer.SummarizeOnOverflow,
		MaxContextMessages:  cfg.Analyzer.MaxContextMessages,
	})
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
			var ln net.Listener
			var err error

			if cfg.HTTP.TLS.CertFile != "" && cfg.HTTP.TLS.KeyFile != "" {
				cert, certErr := tls.LoadX509KeyPair(cfg.HTTP.TLS.CertFile, cfg.HTTP.TLS.KeyFile)
				if certErr != nil {
					return fmt.Errorf("loading TLS cert/key: %w", certErr)
				}
				ln, err = tls.Listen("tcp", addr, &tls.Config{
					Certificates: []tls.Certificate{cert},
					MinVersion:   tls.VersionTLS12,
				})
				if err != nil {
					return fmt.Errorf("failed to listen on %s: %w", addr, err)
				}
				log.Printf("Starting meerkat server on https://%s", addr)
			} else {
				ln, err = net.Listen("tcp", addr)
				if err != nil {
					return fmt.Errorf("failed to listen on %s: %w", addr, err)
				}
				log.Printf("Starting meerkat server on http://%s", addr)
			}

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

func WaitForShutdown(_ *http.Server, lc fx.Lifecycle) {
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
			ProvideToolRegistry,
			ProvideDatasourceRefs,
			insprepo.NewRepository,
			ProvideAnalyzerProvider,
			ProvideDedupWindow,
			ProvideAnalyzerService,
			inspector.NewService,
			ProvideReporterService,
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
