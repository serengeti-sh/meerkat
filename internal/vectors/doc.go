// Package vectors implements the Vectors pipeline for log ingestion,
// semantic search, and contextual retrieval.
//
// It provides log ingestion with template extraction (Drain algorithm), semantic
// search via vector stores (Milvus, Qdrant), and contextual retrieval for
// incident analysis. The package exposes a domain Service interface and a
// GRPCServer transport adapter.
//
// This package is the core library used by the vectors server. Consumers
// (analyzer, collector) should communicate with vectors via gRPC
// (logsclient package) rather than using this package directly.
//
// Entry points:
//
//	svc, err := vectors.NewService(embedder, vectorStore, opts...)
//	server, err := vectors.NewGRPCServer(svc)
package vectors
