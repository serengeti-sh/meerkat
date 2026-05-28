package serve

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/rag"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
	"github.com/serengeti-sh/meerkat/internal/ragpb"
)

// Run starts the RAG gRPC server.
func Run(cfgFile string, port int) error {
	cfg, err := loadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if port != 0 {
		cfg.RAG.Port = port
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

	ragSvc, err := rag.NewService(emb, vstore)
	if err != nil {
		return fmt.Errorf("create rag service: %w", err)
	}
	ragServer, err := rag.NewGRPCServer(ragSvc)
	if err != nil {
		return fmt.Errorf("create rag grpc server: %w", err)
	}

	grpcServer := grpc.NewServer()
	ragpb.RegisterServiceServer(grpcServer, ragServer)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.RAG.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go func() {
		log.Printf("RAG gRPC server listening on :%d", cfg.RAG.Port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)

	sig := <-shutdownCh
	log.Printf("Received signal %v, shutting down...", sig)

	grpcServer.GracefulStop()
	log.Println("RAG server stopped gracefully")
	return nil
}

func loadConfig(cfgFile string) (*config.Config, error) {
	if cfgFile != "" {
		return config.LoadFromPath(cfgFile)
	}
	return config.Load()
}
