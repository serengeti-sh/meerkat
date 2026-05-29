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
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embed"
	"github.com/serengeti-sh/meerkat/internal/httphandler"
	"github.com/serengeti-sh/meerkat/internal/inspect"
	"github.com/serengeti-sh/meerkat/internal/notify"
	"github.com/serengeti-sh/meerkat/internal/report"
	"github.com/serengeti-sh/meerkat/internal/schedule"
	"github.com/serengeti-sh/meerkat/internal/tool"
	"github.com/serengeti-sh/meerkat/internal/vectorsclient"
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
	server  *http.Server
	sched   schedule.Service
	inspect inspect.Service
}

// NewAnalyzer creates an Analyzer with all dependencies wired.
func NewAnalyzer(cfg *config.Config) (*Analyzer, error) {
	// Database
	client, err := inspect.NewEntClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create db client: %w", err)
	}

	if err := inspect.Migrate(context.Background(), client); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("migrate db: %w", err)
	}

	// Repository
	reportRepo := report.NewEntReportRepository(client)

	// Embedder
	emb := embed.New(cfg.Embedder.APIKey, cfg.Embedder.BaseURL, cfg.Embedder.Model)

	// MeerkatLogs client
	logsClient, err := newLogsClient(cfg)
	if err != nil {
		return nil, err
	}

	// Tool registry
	toolRegistry, err := buildToolRegistry(cfg, emb, logsClient)
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
	})

	analyzerSvc, err := buildAnalyzerService(provider, toolRegistry, cfg)
	if err != nil {
		return nil, fmt.Errorf("build analyzer service: %w", err)
	}

	// Reporter
	reporterSvc := notify.NewService(cfg.Reporter.WebhookURL, cfg.Reporter.MinSeverity, nil)

	// Inspector
	var inspectorOpts []inspect.ServiceOption
	if logsClient != nil {
		inspectorOpts = append(inspectorOpts, inspect.WithLogsClient(logsClient))
	}

	inspectorSvc, err := inspect.NewService(
		analyzerSvc,
		reportRepo,
		reporterSvc,
		buildDatasourceRefs(cfg),
		cfg.Inspector.GetDedupWindow(),
		cfg.Inspector.QueueSize,
		cfg.Inspector.WorkerCount,
		inspectorOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("create inspector service: %w", err)
	}

	// HTTP handler
	h, err := httphandler.New(inspectorSvc)
	if err != nil {
		return nil, fmt.Errorf("create http handler: %w", err)
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
	}

	sched := schedule.NewService(inspectorSvc, cfg)

	return &Analyzer{
		cfg:     cfg,
		server:  srv,
		sched:   sched,
		inspect: inspectorSvc,
	}, nil
}

// Run starts the analyzer server and blocks until shutdown.
func (a *Analyzer) Run() error {
	if err := a.inspect.Start(); err != nil {
		return fmt.Errorf("start inspector: %w", err)
	}
	defer a.inspect.Stop()

	addr := a.server.Addr
	var ln net.Listener
	var err error

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
		log.Printf("Starting meerkat server on https://%s", addr)
	} else {
		ln, err = net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("failed to listen on %s: %w", addr, err)
		}
		log.Printf("Starting meerkat server on http://%s", addr)
	}

	go func() { _ = a.server.Serve(ln) }()

	if a.cfg.Scheduler.Enabled {
		log.Println("Starting scheduler...")
		if err := a.sched.Start(context.Background()); err != nil {
			log.Printf("Scheduler error: %v", err)
		}
	}

	// Graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownCh
	log.Printf("Received signal %v, shutting down...", sig)

	a.sched.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Println("Server stopped gracefully")
	return nil
}

func newLogsClient(cfg *config.Config) (vectorsclient.Client, error) {
	if !cfg.Vectors.Enabled || cfg.Vectors.Address == "" {
		return nil, nil
	}
	client, err := vectorsclient.New(cfg.Vectors.Address)
	if err != nil {
		return nil, fmt.Errorf("create vectors client: %w", err)
	}
	return client, nil
}

func buildToolRegistry(cfg *config.Config, emb embed.Model, logsClient vectorsclient.Client) (*tool.Registry, error) {
	var tools []tool.Plugin

	for _, pc := range cfg.Tools.Prometheus {
		httpClient, err := tool.NewHTTPClient(pc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("tool %q: %w", pc.Name, err)
		}
		description := cfg.Tools.PrometheusDescription
		if description == "" {
			description = "Query metrics using PromQL. Returns time series data."
		}
		schemaFile := cfg.Tools.PrometheusParamSchemaFile
		if schemaFile == "" {
			schemaFile = "internal/tool/schemas/prometheus.json"
		}
		t, err := tool.NewPrometheusTool(pc.Name, description, schemaFile, pc.URL, httpClient)
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
		description := cfg.Tools.LokiDescription
		if description == "" {
			description = "Query logs using LogQL. Returns log entries with timestamps and labels."
		}
		schemaFile := cfg.Tools.LokiParamSchemaFile
		if schemaFile == "" {
			schemaFile = "internal/tool/schemas/loki.json"
		}
		t, err := tool.NewLokiTool(lc.Name, description, schemaFile, lc.URL, httpClient)
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
		description := cfg.Tools.VictoriaLogsDescription
		if description == "" {
			description = "Query logs using LogsQL. Returns log entries."
		}
		schemaFile := cfg.Tools.VictoriaLogsParamSchemaFile
		if schemaFile == "" {
			schemaFile = "internal/tool/schemas/victorialogs.json"
		}
		t, err := tool.NewVictoriaLogsTool(vc.Name, description, schemaFile, vc.URL, httpClient)
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

	if logsClient != nil {
		tools = append(tools, tool.NewSearchLogsTool(logsClient))
	}

	return tool.NewRegistry(tools...), nil
}

func buildAnalyzerService(provider analyzer.LLMProvider, registry *tool.Registry, cfg *config.Config) (analyzer.Service, error) {
	systemPrompt, err := analyzer.LoadSystemPrompt(cfg.Analyzer.SystemPromptFile)
	if err != nil {
		return nil, fmt.Errorf("load system prompt: %w", err)
	}
	skills, err := analyzer.LoadSkills(cfg.Analyzer.SkillsFile)
	if err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}
	prompt := analyzer.MergeSkillsIntoPrompt(systemPrompt, skills)
	svc, err := analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations:       cfg.Analyzer.MaxIterations,
		SystemPrompt:        prompt,
		MaxToolResultChars:  cfg.Analyzer.MaxToolResultChars,
		SummarizeOnOverflow: cfg.Analyzer.SummarizeOnOverflow,
		MaxContextMessages:  cfg.Analyzer.MaxContextMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("create analyzer service: %w", err)
	}
	return svc, nil
}

func buildDatasourceRefs(cfg *config.Config) func() []analyzer.DatasourceRef {
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
