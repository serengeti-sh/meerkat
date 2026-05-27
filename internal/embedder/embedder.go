package embedder

import "context"

// Interface converts text into dense vector representations.
type Interface interface {
	// Embed generates embeddings for the given texts.
	// Returns a slice of vectors, one per input text.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// New creates an Interface based on the provider configuration.
// Currently supports "openai".
func New(apiKey, baseURL, model string) Interface {
	return newOpenAIEmbedder(apiKey, baseURL, model)
}
