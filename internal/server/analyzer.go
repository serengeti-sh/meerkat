// Package server assembles and runs the Meerkat services.
//
// Each exported function (e.g. NewAnalyzer) encapsulates
// the dependency wiring for its respective service. This keeps cmd/ thin and
// focused on CLI concerns (flags, config paths, signals).
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/httphandler"
	"github.com/serengeti-sh/meerkat/internal/inspect"
	"github.com/serengeti-sh/meerkat/internal/logger"
	"github.com/serengeti-sh/meerkat/internal/notify"
	"github.com/serengeti-sh/meerkat/internal/report"
	"github.com/serengeti-sh/meerkat/internal/schedule"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	httpWriteTimeout      = 60 * time.Second
	httpIdleTimeout       = 120 * time.Second
	shutdownTimeout       = 30 * time.Second
)

// Analyzer assembles and runs the analyzer HTTP server.
type Analyzer struct {
	cfg     *config.Config
	sched   schedule.Service
	inspect inspect.Service
	handler *httphandler.Handler
	log     zerolog.Logger
}

// NewAnalyzer creates an Analyzer with all dependencies wired.
func NewAnalyzer(ctx context.Context, cfg *config.Config) (*Analyzer, error) {
	logCfg := logger.Config{
		Level:  cfg.App.LogLevel,
		Format: cfg.App.LogFormat,
	}
	if cfg.App.Debug {
		logCfg.Level = "debug"
	}
	log := logger.New(logCfg)
	ctx = logger.WithContext(ctx, log)

	log.Info().Str("version", cfg.App.Version).Msg("initializing analyzer")

	// Database
	client, err := inspect.NewEntClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create db client: %w", err)
	}

	if err := inspect.Migrate(ctx, client); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close database client after migration failure")
		}
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	// Repository
	reportRepo := report.NewEntReportRepository(client)

	// Vectors client
	vectorsClient, err := newVectorsClient(cfg)
	if err != nil {
		return nil, err
	}

	// Tool registry
	toolRegistry, err := buildToolRegistry(cfg, vectorsClient)
	if err != nil {
		return nil, fmt.Errorf("build tool registry: %w", err)
	}

	// Analyzer provider and service
	provider := analyzer.NewLLMProvider(analyzer.ProviderConfig{
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
		Log: log,
	})

	if err := provider.HealthCheck(ctx); err != nil {
		return nil, fmt.Errorf("llm provider health check failed: %w", err)
	}

	analyzerSvc, err := buildAnalyzerService(provider, toolRegistry, cfg, log)
	if err != nil {
		return nil, fmt.Errorf("build analyzer service: %w", err)
	}

	// Reporter
	reporterSvc := notify.NewService(cfg.Notify.WebhookURL, cfg.Notify.MinSeverity, nil, log)

	// Inspector
	var inspectorOpts []inspect.ServiceOption
	if vectorsClient != nil {
		inspectorOpts = append(inspectorOpts, inspect.WithVectorsClient(vectorsClient))
	}

	inspectorSvc, err := inspect.NewService(
		analyzerSvc,
		reportRepo,
		reporterSvc,
		buildDatasourceRefs(cfg),
		cfg.Inspect.GetDedupWindow(),
		cfg.Inspect.QueueSize,
		cfg.Inspect.WorkerCount,
		log,
		inspectorOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("create inspector service: %w", err)
	}

	// HTTP handler
	h := httphandler.New(inspectorSvc, log)

	sched := schedule.NewService(inspectorSvc, cfg, log)

	return &Analyzer{
		cfg:     cfg,
		sched:   sched,
		inspect: inspectorSvc,
		handler: h,
		log:     log,
	}, nil
}

// Run starts the analyzer server and blocks until shutdown.
func (a *Analyzer) Run(ctx context.Context) error {
	if err := a.inspect.Start(); err != nil {
		return fmt.Errorf("start inspector: %w", err)
	}
	defer a.inspect.Stop()

	ogenServer, err := api.NewServer(a.handler, api.WithPathPrefix("/v1"))
	if err != nil {
		return fmt.Errorf("create ogen server: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", a.cfg.HTTP.Host, a.cfg.HTTP.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           ogenServer,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}

	var ln net.Listener
	if a.cfg.HTTP.TLS.CertFile != "" && a.cfg.HTTP.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(a.cfg.HTTP.TLS.CertFile, a.cfg.HTTP.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("loading TLS cert/key: %w", err)
		}
		ln, err = tls.Listen("tcp", addr, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", addr, err)
		}
		a.log.Info().Str("addr", addr).Msg("starting meerkat server")
	} else {
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", addr, err)
		}
		a.log.Info().Str("addr", addr).Msg("starting meerkat server")
	}

	go func() { _ = srv.Serve(ln) }()

	if a.cfg.Schedule.Enabled {
		a.log.Info().Msg("starting scheduler")
		if err := a.sched.Start(ctx); err != nil {
			a.log.Error().Err(err).Msg("scheduler error")
		}
	}

	// Graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownCh
	a.log.Info().Str("signal", sig.String()).Msg("shutting down")

	a.sched.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	a.log.Info().Msg("server stopped gracefully")
	return nil
}
