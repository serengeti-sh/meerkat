// Package scheduler runs periodic inspection jobs using a cron-like ticker.
//
// Each configured job invokes inspector.Service.Inspect at the specified
// interval. The scheduler manages worker goroutine lifecycle and graceful
// shutdown via context cancellation.
package scheduler
