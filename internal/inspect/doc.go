// Package inspect orchestrates report lifecycle: queueing, worker pools,
// deduplication, and optional vectors-based log context enrichment.
//
// The inspect service receives analysis requests (via HTTP handler or scheduler),
// queues them, and dispatches worker goroutines that delegate actual analysis
// to the analyzer service. It persists reports via Repository and
// optionally enriches prompts with vectors log context.
//
// Entry point: NewService(analyzerSvc, reportRepo, reporterSvc, dsRefs, ...)
package inspect
