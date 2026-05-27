// Package collector receives OTLP log exports over gRPC, batches them,
// and flushes to the vector store or RAG pipeline.
//
// It consists of a GRPCServer transport layer, an OTLP handler, and a Batcher
// that implements the LogAdder interface for downstream consumers.
package collector
