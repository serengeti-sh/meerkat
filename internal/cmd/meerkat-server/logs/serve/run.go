package serve

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	mlCfg := cfg.ResolveMeerkatLogs()
	if port != 0 {
		mlCfg.Port = port
	}

	// Override vector store retention if meerkat_logs.retention is set.
	if mlCfg.Retention > 0 {
		cfg.VectorStore.Milvus.Retention = mlCfg.Retention
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

	// Create RAG service with configurable threshold.
	ragOpts := []rag.ServiceOption{}
	if mlCfg.SimilarityThreshold > 0 {
		ragOpts = append(ragOpts, rag.WithSimilarityThreshold(mlCfg.SimilarityThreshold))
	}
	if mlCfg.IngestBatchSize > 0 {
		ragOpts = append(ragOpts, rag.WithBatchSize(mlCfg.IngestBatchSize))
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

	grpcAddr := mlCfg.Address
	if grpcAddr == "" {
		grpcAddr = fmt.Sprintf(":%d", mlCfg.Port)
	}
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

	// Start OTLP receiver for log ingestion.
	var otlpServer *collector.GRPCServer
	if mlCfg.OTLPBindAddr != "" {
		batcher := collector.NewBatcher(cfg, emb, vstore).WithRAGService(ragSvc)
		batcher.Start()
		defer batcher.Stop(context.Background())

		// Collector config uses OTLPBindAddr from meerkat_logs.
		collectorCfg := *cfg
		collectorCfg.Collector.OTLPBindAddr = mlCfg.OTLPBindAddr

		otlpServer = collector.NewGRPCServer(&collectorCfg, batcher)
		if err := otlpServer.Start(); err != nil {
			return fmt.Errorf("start otlp receiver: %w", err)
		}
		log.Printf("MeerkatLogs OTLP receiver listening on %s", mlCfg.OTLPBindAddr)
		defer otlpServer.Stop()
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownCh
	log.Printf("Received signal %v, shutting down...", sig)

	// Graceful shutdown with timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcServer.GracefulStop()
	if otlpServer != nil {
		otlpServer.Stop()
	}

	log.Println("MeerkatLogs server stopped gracefully")
	_ = shutdownCtx
	return nil
}

func loadConfig(cfgFile string) (*config.Config, error) {
	if cfgFile != "" {
		return config.LoadFromPath(cfgFile)
	}
	return config.Load()
}
