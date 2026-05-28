// Package logsclient provides a gRPC client for the MeerkatLogs service.
//
// The client abstracts the underlying protobuf types and exposes domain-specific
// types such as LogEntry, SearchResult, and SearchOptions.
//
// Usage:
//
//	client, err := logsclient.New("localhost:50051")
//	if err != nil {
//	    // handle error
//	}
//	defer client.Close()
//
//	result, err := client.Search(ctx, "database error", logsclient.SearchOptions{Limit: 10})
package logsclient
