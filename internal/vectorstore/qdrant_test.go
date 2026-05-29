package vectorstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

func TestAPIKeyAuth_GetRequestMetadata(t *testing.T) {
	auth := vectorstore.NewAPIKeyAuth("test-api-key")
	
	metadata, err := auth.GetRequestMetadata(context.Background(), "test-uri")
	require.NoError(t, err)
	assert.Equal(t, "test-api-key", metadata["api-key"])
}

func TestAPIKeyAuth_RequireTransportSecurity(t *testing.T) {
	auth := vectorstore.NewAPIKeyAuth("test-api-key")
	assert.False(t, auth.RequireTransportSecurity())
}

func TestJoinQuoted(t *testing.T) {
	tests := []struct {
		name     string
		ids      []string
		expected string
	}{
		{
			name:     "single id",
			ids:      []string{"id1"},
			expected: `"id1"`,
		},
		{
			name:     "multiple ids",
			ids:      []string{"id1", "id2", "id3"},
			expected: `"id1", "id2", "id3"`,
		},
		{
			name:     "empty ids",
			ids:      []string{},
			expected: "",
		},
		{
			name:     "ids with special characters",
			ids:      []string{"id-1", "id_2", "id.3"},
			expected: `"id-1", "id_2", "id.3"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vectorstore.JoinQuoted(tt.ids)
			assert.Equal(t, tt.expected, result)
		})
	}
}
