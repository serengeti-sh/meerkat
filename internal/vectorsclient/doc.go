// Package vectorsclient provides a gRPC client for the Vectors service.
//
// The client abstracts the underlying protobuf types and exposes domain-specific
// types such as Entry, SearchResult, and SearchOptions.
//
// Usage:
//
//	client, err := vectorsclient.New("localhost:50051")
//	if err != nil {
//	    // handle error
//	}
//	defer client.Close()
//
//	result, err := client.Search(ctx, "database error", vectorsclient.SearchOptions{Limit: 10})
package vectorsclient
