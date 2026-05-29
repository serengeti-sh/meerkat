package vectorstore_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

func TestNew_UnsupportedDriver(t *testing.T) {
	cfg := &config.Config{
		VectorStore: config.VectorStoreConfig{
			Driver: "unsupported",
		},
	}

	_, err := vectorstore.New(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported vector store driver")
}
