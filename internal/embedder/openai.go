package embedder

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// openAIEmbedder implements Embedder using the official OpenAI Go SDK v3.
type openAIEmbedder struct {
	client openai.Client
	model  string
}

func newOpenAIEmbedder(apiKey, baseURL, model string) *openAIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}

	opts := []option.RequestOption{}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(strings.TrimRight(baseURL, "/")+"/v1"))
	}

	return &openAIEmbedder{
		client: openai.NewClient(opts...),
		model:  model,
	}
}

func (e *openAIEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	resp, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: texts,
		},
		Model: e.model,
	})
	if err != nil {
		return nil, fmt.Errorf("create embeddings: %w", err)
	}

	vectors := make([][]float32, len(texts))
	for _, d := range resp.Data {
		idx := int(d.Index)
		if idx < 0 || idx >= len(vectors) {
			continue
		}
		vec := make([]float32, len(d.Embedding))
		for i, v := range d.Embedding {
			vec[i] = float32(v)
		}
		vectors[idx] = vec
	}

	for i, v := range vectors {
		if v == nil {
			return nil, fmt.Errorf("missing embedding for text at index %d", i)
		}
	}

	return vectors, nil
}
