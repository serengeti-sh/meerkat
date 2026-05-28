// Package collector receives OTLP log exports over gRPC, batches them,
// and flushes to the MeerkatLogs server.
//
// It consists of a GRPCServer transport layer, an OTLP handler, and a Batcher
// that implements the LogSink interface for downstream consumers.
package collector
