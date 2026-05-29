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
	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embed"
	"github.com/serengeti-sh/meerkat/internal/meerkatlogspb"
	"github.com/serengeti-sh/meerkat/internal/vectors"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

// Run starts the Vectors gRPC server (Search/Ingest/GetContext).
func Run(cfgFile string, port int) error {
	cfg, err := loadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	ml := cfg.Vectors
	if port != 0 {
		ml.Port = port
	}

	// Override vector store retention if meerkat_logs.retention is set.
	if ml.Retention > 0 {
		cfg.VectorStore.Milvus.Retention = ml.Retention
	}

	emb := embed.New(cfg.Embedder.APIKey, cfg.Embedder.BaseURL, cfg.Embedder.Model)

	vstore, err := vectorstore.New(cfg)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}
	defer func() {
		if err := vstore.Close(); err != nil {
			log.Printf("failed to close vector store: %v", err)
		}
	}()

	// Create Vectors service with configurable threshold and filtering.
	logsOpts := []vectors.ServiceOption{
		vectors.WithFilterMode(ml.FilterMode, ml.MinSeverity),
	}
	if ml.SimilarityThreshold > 0 {
		logsOpts = append(logsOpts, vectors.WithSimilarityThreshold(ml.SimilarityThreshold))
	}
	if ml.IngestBatchSize > 0 {
		logsOpts = append(logsOpts, vectors.WithBatchSize(ml.IngestBatchSize))
	}
	logsSvc, err := vectors.NewService(emb, vstore, logsOpts...)
	if err != nil {
		return fmt.Errorf("create vectors service: %w", err)
	}

	// Start gRPC server for Search/Ingest/GetContext.
	logsServer, err := vectors.NewGRPCServer(logsSvc)
	if err != nil {
		return fmt.Errorf("create vectors grpc server: %w", err)
	}

	grpcServer := grpc.NewServer()
	meerkatlogspb.RegisterServiceServer(grpcServer, logsServer)

	// Register OTLP logs receiver.
	otlpServer := vectors.NewOTLPServer(logsSvc)
	logsv1.RegisterLogsServiceServer(grpcServer, otlpServer)

	grpcAddr := ml.GetAddress()
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	go func() {
		log.Printf("Vectors gRPC server listening on %s (includes OTLP)", grpcAddr)
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
		log.Printf("Vectors metrics server listening on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("metrics server error: %v", err)
		}
	}()

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
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("metrics server shutdown error: %v", err)
	}

	log.Println("Vectors server stopped gracefully")
	return nil
}

func loadConfig(cfgFile string) (*config.Config, error) {
	if cfgFile != "" {
		return config.LoadFromPath(cfgFile)
	}
	return config.Load()
}
