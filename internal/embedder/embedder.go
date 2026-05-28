package embedder

import "context"

// Embedder converts text into dense vector representations.
type Embedder interface {
	// Embed generates embeddings for the given texts.
	// Returns a slice of vectors, one per input text.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// New creates an Embedder based on the provider configuration.
// Currently supports "openai".
func New(apiKey, baseURL, model string) Embedder {
	return newOpenAIEmbedder(apiKey, baseURL, model)
}
