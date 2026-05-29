// Package schedule runs periodic inspection jobs using a cron-like ticker.
//
// Each configured job invokes inspect.Service.Inspect at the specified
// interval. The scheduler manages worker goroutine lifecycle and graceful
// shutdown via context cancellation.
package schedule
