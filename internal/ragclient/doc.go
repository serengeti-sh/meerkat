// Package ragclient provides a gRPC client for the Meerkat RAG (Retrieval-Augmented Generation) service.
//
// The client abstracts the underlying protobuf types and exposes domain-specific
// types such as LogEntry, SearchResult, and SearchOptions.
//
// Usage:
//
//	client, err := ragclient.New("localhost:50051")
//	if err != nil {
//	    // handle error
//	}
//	defer client.Close()
//
//	result, err := client.Search(ctx, "database error", ragclient.SearchOptions{Limit: 10})
package ragclient
