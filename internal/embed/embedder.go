package embed

import "context"

// Model converts text into dense vector representations.
type Model interface {
	// Embed generates embeddings for the given texts.
	// Returns a slice of vectors, one per input text.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// New creates a Model based on the provider configuration.
// Currently supports "openai".
func New(apiKey, baseURL, model string) *openAIEmbedder {
	return newOpenAIEmbedder(apiKey, baseURL, model)
}
