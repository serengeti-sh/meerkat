// Package logstream processes real-time log streams from VictoriaLogs.
//
// It provides a Connector for tailing log streams, a Processor with bounded
// worker pools for threshold-based alerting, and a SlidingWindow for time-window
// aggregation.
package logstream
