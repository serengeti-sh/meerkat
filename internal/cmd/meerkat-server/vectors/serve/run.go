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

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embed"
	"github.com/serengeti-sh/meerkat/internal/vectors"
	"github.com/serengeti-sh/meerkat/internal/vectorspb"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

// Run starts the Vectors server with OTLP ingestion and query services.
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

	// Override vector store retention if vectors.retention is set.
	if ml.Retention > 0 {
		cfg.VectorStore.Milvus.Retention = ml.Retention
	}

	emb := embed.New(cfg.Embed.APIKey, cfg.Embed.BaseURL, cfg.Embed.Model)

	vstore, err := vectorstore.New(cfg)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}
	defer func() {
		if err := vstore.Close(); err != nil {
			log.Printf("failed to close vector store: %v", err)
		}
	}()

	// Health checks
	ctx := context.Background()
	if err := vstore.Ping(ctx); err != nil {
		return fmt.Errorf("vector store connection failed: %w", err)
	}
	if err := emb.HealthCheck(ctx); err != nil {
		return fmt.Errorf("embedder health check failed: %w", err)
	}

	// Create Vectors service with configurable threshold and filtering.
	vectorsOpts := []vectors.ServiceOption{
		vectors.WithFilterMode(ml.FilterMode, ml.MinSeverity),
	}
	if ml.SimilarityThreshold > 0 {
		vectorsOpts = append(vectorsOpts, vectors.WithSimilarityThreshold(ml.SimilarityThreshold))
	}
	if ml.IngestBatchSize > 0 {
		vectorsOpts = append(vectorsOpts, vectors.WithBatchSize(ml.IngestBatchSize))
	}
	vectorsSvc, err := vectors.NewService(emb, vstore, vectorsOpts...)
	if err != nil {
		return fmt.Errorf("create vectors service: %w", err)
	}

	// Start ingestors (currently OTLP only, extensible for Kafka, HTTP, file, etc.)
	ingestors := []vectors.Ingestor{
		vectors.NewOTLPIngestor(ml.GetAddress()),
	}
	for _, ing := range ingestors {
		if err := ing.Start(context.Background(), vectorsSvc); err != nil {
			return fmt.Errorf("start ingestor %q: %w", ing.Name(), err)
		}
	}

	// Start gRPC server for Search/GetContext.
	vectorsServer, err := vectors.NewGRPCServer(vectorsSvc)
	if err != nil {
		return fmt.Errorf("create vectors grpc server: %w", err)
	}

	grpcServer := grpc.NewServer()
	vectorspb.RegisterServiceServer(grpcServer, vectorsServer)

	grpcAddr := ml.GetAddress()
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	go func() {
		log.Printf("Vectors query gRPC server listening on %s", grpcAddr)
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

	// Stop ingestors
	for _, ing := range ingestors {
		if err := ing.Stop(shutdownCtx); err != nil {
			log.Printf("ingestor %q stop error: %v", ing.Name(), err)
		}
	}

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
