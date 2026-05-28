package serve

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"

	"github.com/serengeti-sh/meerkat/internal/collector"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/rag"
	"github.com/serengeti-sh/meerkat/internal/ragpb"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

// Run starts the MeerkatLogs gRPC server (Search/Ingest/GetContext) and OTLP receiver.
func Run(cfgFile string, port int) error {
	cfg, err := loadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	ml := cfg.MeerkatLogs
	if port != 0 {
		ml.Port = port
	}

	// Override vector store retention if meerkat_logs.retention is set.
	if ml.Retention > 0 {
		cfg.VectorStore.Milvus.Retention = ml.Retention
	}

	emb := embedder.New(cfg.Embedder.APIKey, cfg.Embedder.BaseURL, cfg.Embedder.Model)

	vstore, err := vectorstore.New(cfg)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}
	defer func() {
		if err := vstore.Close(); err != nil {
			log.Printf("failed to close vector store: %v", err)
		}
	}()

	// Create RAG service with configurable threshold and filtering.
	ragOpts := []rag.ServiceOption{
		rag.WithFilterMode(ml.FilterMode, ml.MinSeverity),
	}
	if ml.SimilarityThreshold > 0 {
		ragOpts = append(ragOpts, rag.WithSimilarityThreshold(ml.SimilarityThreshold))
	}
	if ml.IngestBatchSize > 0 {
		ragOpts = append(ragOpts, rag.WithBatchSize(ml.IngestBatchSize))
	}
	ragSvc, err := rag.NewService(emb, vstore, ragOpts...)
	if err != nil {
		return fmt.Errorf("create rag service: %w", err)
	}

	// Start gRPC server for Search/Ingest/GetContext.
	ragServer, err := rag.NewGRPCServer(ragSvc)
	if err != nil {
		return fmt.Errorf("create rag grpc server: %w", err)
	}

	grpcServer := grpc.NewServer()
	ragpb.RegisterServiceServer(grpcServer, ragServer)

	grpcAddr := ml.GetAddress()
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	go func() {
		log.Printf("MeerkatLogs gRPC server listening on %s", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// Start HTTP server for metrics.
	httpMux := http.NewServeMux()
	httpMux.Handle("/metrics", promhttp.Handler())
	httpMux.Handle("/healthz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))

	httpAddr := ":9090"
	if ml.Port != 0 {
		httpAddr = fmt.Sprintf(":%d", ml.Port+1000)
	}
	httpServer := &http.Server{Addr: httpAddr, Handler: httpMux}
	go func() {
		log.Printf("MeerkatLogs metrics server listening on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("metrics server error: %v", err)
		}
	}()

	// Start OTLP receiver for log ingestion.
	var otlpServer *collector.GRPCServer
	if ml.OTLPBindAddr != "" {
		batcher := collector.NewBatcher(cfg, emb, vstore).WithRAGService(ragSvc)
		batcher.Start()
		defer batcher.Stop(context.Background())

		// Collector config uses OTLPBindAddr from meerkat_logs.
		collectorCfg := *cfg
		collectorCfg.Collector.OTLPBindAddr = ml.OTLPBindAddr

		otlpServer = collector.NewGRPCServer(&collectorCfg, batcher)
		if err := otlpServer.Start(); err != nil {
			return fmt.Errorf("start otlp receiver: %w", err)
		}
		log.Printf("MeerkatLogs OTLP receiver listening on %s", ml.OTLPBindAddr)
		defer otlpServer.Stop()
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownCh
	log.Printf("Received signal %v, shutting down...", sig)

	// Graceful shutdown with timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	if otlpServer != nil {
		otlpServer.Stop()
	}
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("metrics server shutdown error: %v", err)
	}

	log.Println("MeerkatLogs server stopped gracefully")
	return nil
}

func loadConfig(cfgFile string) (*config.Config, error) {
	if cfgFile != "" {
		return config.LoadFromPath(cfgFile)
	}
	return config.Load()
}
