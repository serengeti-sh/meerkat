// Package inspector orchestrates report lifecycle: queueing, worker pools,
// deduplication, and optional RAG-based log context enrichment.
//
// The inspector receives analysis requests (via HTTP handler or scheduler),
// queues them, and dispatches worker goroutines that delegate actual analysis
// to the analyzer service. It persists reports via ReportRepository and
// optionally enriches prompts with RAG log context.
//
// Entry point: NewService(analyzerSvc, reportRepo, reporterSvc, dsRefs, ...)
package inspector
