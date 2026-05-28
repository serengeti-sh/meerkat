// Package rag implements the Retrieval-Augmented Generation (RAG) pipeline.
//
// It provides log ingestion with template extraction (Drain algorithm), semantic
// search via vector stores (Milvus, Qdrant), and contextual retrieval for
// incident analysis. The package exposes a domain Service interface and a
// GRPCServer transport adapter.
//
// Entry points:
//
//	svc, err := rag.NewService(embedder, vectorStore)
//	server, err := rag.NewGRPCServer(svc)
package rag
