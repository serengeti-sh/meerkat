// Package embedder converts text into dense vector embeddings for semantic search.
//
// It abstracts the underlying embedding provider (currently OpenAI-compatible)
// behind a simple Model interface so the rest of the codebase remains
// provider-agnostic.
package embed
