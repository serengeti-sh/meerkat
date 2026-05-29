package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/serengeti-sh/meerkat/internal/vectors"
	"github.com/serengeti-sh/meerkat/internal/vectorspb"
)

// TestE2E_VectorsPipeline tests the complete vectors service pipeline:
// 1. Ingest logs via the service layer
// 2. Search for similar logs via gRPC
// 3. Get context for a time range via gRPC
func TestE2E_VectorsPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()

	// Setup: create service with mock embedder and in-memory vector store
	emb := &mockEmbedder{
		vectors: [][]float32{
			{0.1, 0.2, 0.3},
			{0.15, 0.25, 0.35},
			{0.8, 0.9, 1.0},
		},
	}
	vs := &inMemoryVectorStore{}

	vectorsSvc, err := vectors.NewService(emb, vs,
		vectors.WithFilterMode("all", ""),
		vectors.WithBatchSize(10),
	)
	require.NoError(t, err)

	// Step 1: Create gRPC server
	grpcServer, err := vectors.NewGRPCServer(vectorsSvc)
	require.NoError(t, err)

	// Step 2: Ingest log entries
	entries := []vectors.Entry{
		{
			Timestamp: time.Now().Add(-30 * time.Minute),
			Service:   "api-server",
			Severity:  "ERROR",
			Body:      "connection refused to database",
		},
		{
			Timestamp: time.Now().Add(-25 * time.Minute),
			Service:   "api-server",
			Severity:  "ERROR",
			Body:      "connection refused to redis cache",
		},
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Service:   "worker",
			Severity:  "INFO",
			Body:      "job completed successfully",
		},
	}

	result, err := vectorsSvc.Ingest(ctx, entries)
	require.NoError(t, err)
	assert.Greater(t, result.IngestedCount, 0, "expected some entries to be ingested")

	// Step 3: Search via gRPC server
	searchResp, err := grpcServer.Search(ctx, &vectorspb.SearchRequest{
		Query:            "database connection error",
		Limit:            10,
		Service:          "api-server",
		Severity:         "ERROR",
		TimeRangeSeconds: 3600,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, searchResp.Results, "expected search results")

	// Step 4: GetContext via gRPC server
	now := time.Now()
	contextResp, err := grpcServer.GetContext(ctx, &vectorspb.GetContextRequest{
		Service:   "api-server",
		StartTime: timestamppb.New(now.Add(-time.Hour)),
		EndTime:   timestamppb.New(now),
		Limit:     10,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, contextResp.Results, "expected context results")
}

// TestE2E_VectorsDeduplication tests template-based deduplication
func TestE2E_VectorsDeduplication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()

	emb := &mockEmbedder{
		vectors: [][]float32{{0.1, 0.2}},
	}
	vs := &inMemoryVectorStore{}

	// Use template deduplication mode
	vectorsSvc, err := vectors.NewService(emb, vs,
		vectors.WithFilterMode("template", ""),
		vectors.WithSimilarityThreshold(0.7),
	)
	require.NoError(t, err)

	// Ingest duplicate log messages
	entries := []vectors.Entry{
		{Timestamp: time.Now(), Service: "api", Severity: "ERROR", Body: "connection refused to db-1"},
		{Timestamp: time.Now(), Service: "api", Severity: "ERROR", Body: "connection refused to db-2"},
		{Timestamp: time.Now(), Service: "api", Severity: "ERROR", Body: "connection refused to db-3"},
	}

	result, err := vectorsSvc.Ingest(ctx, entries)
	require.NoError(t, err)

	// With template deduplication, only the first unique template should be ingested
	assert.Equal(t, 1, result.IngestedCount, "expected only 1 unique template to be ingested")
	assert.Equal(t, 2, result.DeduplicatedCount, "expected 2 duplicates")
}

// TestE2E_VectorsSeverityFiltering tests severity-based filtering
func TestE2E_VectorsSeverityFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()

	emb := &mockEmbedder{
		vectors: [][]float32{{0.1, 0.2}},
	}
	vs := &inMemoryVectorStore{}

	// Filter out INFO and below, only WARNING and above
	vectorsSvc, err := vectors.NewService(emb, vs,
		vectors.WithFilterMode("severity", "warning"),
	)
	require.NoError(t, err)

	entries := []vectors.Entry{
		{Timestamp: time.Now(), Service: "api", Severity: "INFO", Body: "request processed"},
		{Timestamp: time.Now(), Service: "api", Severity: "WARNING", Body: "high latency detected"},
		{Timestamp: time.Now(), Service: "api", Severity: "ERROR", Body: "connection failed"},
		{Timestamp: time.Now(), Service: "api", Severity: "DEBUG", Body: "debug trace"},
	}

	result, err := vectorsSvc.Ingest(ctx, entries)
	require.NoError(t, err)

	assert.Equal(t, 2, result.IngestedCount, "expected only WARNING and ERROR to be ingested")
	assert.Equal(t, 2, result.FilteredCount, "expected INFO and DEBUG to be filtered")
}
