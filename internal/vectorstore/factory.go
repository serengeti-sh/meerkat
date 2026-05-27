package vectorstore

import (
	"fmt"

	"github.com/serengeti-sh/meerkat/internal/config"
)

// New creates a Store based on the configured driver.
func New(cfg *config.Config) (Store, error) {
	switch cfg.VectorStore.Driver {
	case "qdrant":
		return NewQdrantClient(cfg)
	case "milvus", "":
		return NewMilvusClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported vector store driver: %q", cfg.VectorStore.Driver)
	}
}
