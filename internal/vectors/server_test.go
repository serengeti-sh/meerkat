package vectors_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/serengeti-sh/meerkat/internal/vectors"
	"github.com/serengeti-sh/meerkat/internal/vectorspb"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

func TestGRPCServer_Search(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1, 0.2}}}
	vs := &mockVectorStore{
		results: []vectorstore.SearchResult{
			{ID: "1", Body: "error: connection failed", Service: "api", Severity: "ERROR", Score: 0.95},
			{ID: "2", Body: "timeout", Service: "api", Severity: "WARN", Score: 0.85},
		},
	}
	svc, err := vectors.NewService(emb, vs)
	require.NoError(t, err)

	server, err := vectors.NewGRPCServer(svc)
	require.NoError(t, err)

	t.Run("successful search", func(t *testing.T) {
		resp, err := server.Search(context.Background(), &vectorspb.SearchRequest{
			Query:            "connection error",
			Limit:            5,
			Service:          "api",
			Severity:         "ERROR",
			TimeRangeSeconds: 3600,
		})
		require.NoError(t, err)
		require.Len(t, resp.Results, 2)
		assert.Equal(t, "error: connection failed", resp.Results[0].Body)
		assert.Equal(t, "api", resp.Results[0].Service)
		assert.Equal(t, "ERROR", resp.Results[0].Severity)
	})

	t.Run("empty query", func(t *testing.T) {
		_, err := server.Search(context.Background(), &vectorspb.SearchRequest{
			Query: "",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty query")
	})

	t.Run("no results", func(t *testing.T) {
		emptyVS := &mockVectorStore{results: nil}
		emptySvc, err := vectors.NewService(emb, emptyVS)
		require.NoError(t, err)

		emptyServer, err := vectors.NewGRPCServer(emptySvc)
		require.NoError(t, err)

		resp, err := emptyServer.Search(context.Background(), &vectorspb.SearchRequest{
			Query: "nonexistent",
		})
		require.NoError(t, err)
		assert.Empty(t, resp.Results)
	})
}

func TestGRPCServer_GetContext(t *testing.T) {
	emb := &mockEmbedder{vectors: [][]float32{{0.1, 0.2}}}
	vs := &mockVectorStore{
		results: []vectorstore.SearchResult{
			{ID: "1", Body: "error: connection failed", Service: "api", Severity: "ERROR", Timestamp: time.Now()},
		},
	}
	svc, err := vectors.NewService(emb, vs)
	require.NoError(t, err)

	server, err := vectors.NewGRPCServer(svc)
	require.NoError(t, err)

	t.Run("successful context retrieval", func(t *testing.T) {
		now := time.Now()
		resp, err := server.GetContext(context.Background(), &vectorspb.GetContextRequest{
			Service:   "api",
			StartTime: timestamppb.New(now.Add(-time.Hour)),
			EndTime:   timestamppb.New(now),
			Limit:     10,
		})
		require.NoError(t, err)
		require.Len(t, resp.Results, 1)
		assert.Equal(t, "api", resp.Results[0].Service)
	})

	t.Run("invalid time range", func(t *testing.T) {
		now := time.Now()
		_, err := server.GetContext(context.Background(), &vectorspb.GetContextRequest{
			Service:   "api",
			StartTime: timestamppb.New(now),
			EndTime:   timestamppb.New(now.Add(-time.Hour)),
			Limit:     10,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid time range")
	})
}

func TestGRPCServer_NewGRPCServer_NilService(t *testing.T) {
	_, err := vectors.NewGRPCServer(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "svc is required")
}
