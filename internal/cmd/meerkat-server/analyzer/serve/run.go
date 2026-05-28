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
	"github.com/serengeti-sh/meerkat/internal/httphandler"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	"github.com/serengeti-sh/meerkat/internal/ragclient"
	"github.com/serengeti-sh/meerkat/internal/report"
	"github.com/serengeti-sh/meerkat/internal/reporter"
	"github.com/serengeti-sh/meerkat/internal/scheduler"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

const (
	defaultHTTPPort       = 8080
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
	client, err := inspector.NewEntClient(cfg)
	if err != nil {
		return fmt.Errorf("create db client: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close db client: %v", err)
		}
	}()

	if err := inspector.Migrate(context.Background(), client); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	// 3. Repository
	reportRepo := report.NewEntReportRepository(client)

	// 4. Embedder
	emb := embedder.New(cfg.Embedder.APIKey, cfg.Embedder.BaseURL, cfg.Embedder.Model)

	// 5. Vector store
	var vstore vectorstore.Store
	if cfg.VectorStore.Driver != "" {
		vstore, err = vectorstore.New(cfg)
		if err != nil {
			return fmt.Errorf("create vector store: %w", err)
		}
		defer func() {
			if err := vstore.Close(); err != nil {
				log.Printf("failed to close vector store: %v", err)
			}
		}()
	}

	// 6. Tool registry
	toolRegistry, err := buildToolRegistry(cfg, emb, vstore)
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
	inspectorSvc, err := inspector.NewService(
		analyzerSvc,
		reportRepo,
		reporterSvc,
		dsRefs,
		cfg.Inspector.GetDedupWindow(),
		cfg.Inspector.QueueSize,
		cfg.Inspector.WorkerCount,
		inspectorOpts...,
	)
	if err != nil {
		return fmt.Errorf("create inspector service: %w", err)
	}
	if err := inspectorSvc.Start(); err != nil {
		return fmt.Errorf("start inspector service: %w", err)
	}
	defer inspectorSvc.Stop()

	// 13. Scheduler
	sched := scheduler.NewService(inspectorSvc, cfg)

	// 14. HTTP handler
	h, err := httphandler.New(inspectorSvc)
	if err != nil {
		return fmt.Errorf("create http handler: %w", err)
	}

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

	if port != defaultHTTPPort {
		cfg.HTTP.Port = port
	}

	return cfg, nil
}
