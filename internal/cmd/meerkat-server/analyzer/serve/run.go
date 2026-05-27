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

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/handler"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	insprepo "github.com/serengeti-sh/meerkat/internal/inspector/repository"
	"github.com/serengeti-sh/meerkat/internal/rag"
	"github.com/serengeti-sh/meerkat/internal/reporter"
	"github.com/serengeti-sh/meerkat/pkg/ragclient"
	"github.com/serengeti-sh/meerkat/internal/scheduler"
	"github.com/serengeti-sh/meerkat/internal/store"
	"github.com/serengeti-sh/meerkat/internal/tool"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

const (
	httpReadHeaderTimeout = 10 * time.Second
	httpReadTimeout       = 30 * time.Second
	httpWriteTimeout      = 60 * time.Second
	httpIdleTimeout       = 120 * time.Second
	shutdownTimeout       = 30 * time.Second
)

// Run starts the analyzer server with manual dependency wiring.
func Run(cfgFile string, port int) error {
	// 1. Load config
	cfg, err := loadConfig(cfgFile, port)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Database
	client, err := store.NewEntClient(cfg)
	if err != nil {
		return fmt.Errorf("create db client: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close db client: %v", err)
		}
	}()

	if err := store.Migrate(context.Background(), client); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	// 3. Repository
	reportRepo := insprepo.NewRepository(client)

	// 4. Embedder
	emb := embedder.New(cfg.Embedder.APIKey, cfg.Embedder.BaseURL, cfg.Embedder.Model)

	// 5. Vector store
	var vs vectorstore.Store
	if cfg.VectorStore.Milvus.Address != "" {
		vs, err = vectorstore.NewMilvusClient(cfg)
		if err != nil {
			return fmt.Errorf("create vector store: %w", err)
		}
		defer func() {
			if err := vs.Close(); err != nil {
				log.Printf("failed to close vector store: %v", err)
			}
		}()
	}

	// 6. Tool registry
	toolRegistry, err := buildToolRegistry(cfg, emb, vs)
	if err != nil {
		return fmt.Errorf("build tool registry: %w", err)
	}

	// 7. Analyzer provider
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

	// 8. Analyzer service
	analyzerSvc, err := buildAnalyzerService(provider, toolRegistry, cfg)
	if err != nil {
		return fmt.Errorf("build analyzer service: %w", err)
	}

	// 9. Reporter service
	reporterSvc := reporter.NewService(cfg.Reporter.WebhookURL, cfg.Reporter.MinSeverity, nil)

	// 10. Datasource refs
	dsRefs := buildDatasourceRefs(cfg)

	// 11. RAG client (optional — for online log retrieval)
	var inspectorOpts []inspector.ServiceOption
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
		inspectorOpts = append(inspectorOpts, inspector.WithRAGClient(ragCli))
	}

	// 12. Inspector service
	inspectorSvc := inspector.NewService(
		analyzerSvc,
		reportRepo,
		reporterSvc,
		dsRefs,
		cfg.Inspector.GetDedupWindow(),
		cfg.Inspector.QueueSize,
		cfg.Inspector.WorkerCount,
		inspectorOpts...,
	)
	defer inspectorSvc.Stop()

	// 13. Scheduler
	sched := scheduler.NewCronScheduler(inspectorSvc, cfg)

	// 14. HTTP handler
	h := handler.NewHandler(inspectorSvc)

	// 15. HTTP server
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

	// Start server
	var ln net.Listener
	if cfg.HTTP.TLS.CertFile != "" && cfg.HTTP.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.HTTP.TLS.CertFile, cfg.HTTP.TLS.KeyFile)
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

	go func() { _ = srv.Serve(ln) }()

	// Start scheduler if enabled
	if cfg.Scheduler.Enabled {
		log.Println("Starting scheduler...")
		if err := sched.Start(context.Background()); err != nil {
			log.Printf("Scheduler error: %v", err)
		}
	}

	// Graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownCh
	log.Printf("Received signal %v, shutting down...", sig)

	sched.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Println("Server stopped gracefully")
	return nil
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

	if port != 8080 {
		cfg.HTTP.Port = port
	}

	return cfg, nil
}

func buildToolRegistry(cfg *config.Config, emb embedder.Embedder, vs vectorstore.Store) (*analyzer.ToolRegistry, error) {
	var tools []tool.Interface

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
			schemaFile = "resources/schemas/prometheus.json"
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
			schemaFile = "resources/schemas/loki.json"
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
			schemaFile = "resources/schemas/victorialogs.json"
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

	if vs != nil {
		tools = append(tools, tool.NewSearchLogsTool(emb, vs))
	}

	if cfg.RAG.Enabled && vs != nil {
		ragSvc := rag.NewService(emb, vs)
		tools = append(tools, tool.NewSearchRAGTool(ragSvc))
	}

	return analyzer.NewToolRegistry(tools...), nil
}

func buildAnalyzerService(provider analyzer.LLMProvider, registry *analyzer.ToolRegistry, cfg *config.Config) (analyzer.Service, error) {
	systemPrompt, err := analyzer.LoadSystemPrompt(cfg.Analyzer.SystemPromptFile)
	if err != nil {
		return nil, fmt.Errorf("load system prompt: %w", err)
	}
	skills, err := analyzer.LoadSkills(cfg.Analyzer.SkillsFile)
	if err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}
	prompt := analyzer.MergeSkillsIntoPrompt(systemPrompt, skills)
	return analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations:       cfg.Analyzer.MaxIterations,
		SystemPrompt:        prompt,
		MaxToolResultChars:  cfg.Analyzer.MaxToolResultChars,
		SummarizeOnOverflow: cfg.Analyzer.SummarizeOnOverflow,
		MaxContextMessages:  cfg.Analyzer.MaxContextMessages,
	}), nil
}

func buildDatasourceRefs(cfg *config.Config) inspector.DatasourceRefs {
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
