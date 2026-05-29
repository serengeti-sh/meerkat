// Package vectorstore abstracts vector database operations for semantic search.
//
// It supports multiple backends (Milvus, Qdrant) via a unified Store interface.
// The factory function New selects the appropriate driver based on configuration.
//
// Entry point:
//
//	store, err := vectorstore.New(cfg)
package vectorstore
