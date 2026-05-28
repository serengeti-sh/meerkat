// Package meerkatlogs implements the MeerkatLogs pipeline for
// MeerkatLogs.
//
// It provides log ingestion with template extraction (Drain algorithm), semantic
// search via vector stores (Milvus, Qdrant), and contextual retrieval for
// incident analysis. The package exposes a domain Service interface and a
// GRPCServer transport adapter.
//
// This package is the core library used by the meerkatlogs server. Consumers
// (analyzer, collector) should communicate with meerkatlogs via gRPC
// (ragclient package) rather than using this package directly.
//
// Entry points:
//
//	svc, err := meerkatlogs.NewService(embedder, vectorStore, opts...)
//	server, err := meerkatlogs.NewGRPCServer(svc)
package meerkatlogs
