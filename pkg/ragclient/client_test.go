package ragclient_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/serengeti-sh/meerkat/internal/rag"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
	"github.com/serengeti-sh/meerkat/pkg/ragclient"
	"github.com/serengeti-sh/meerkat/pkg/ragpb"
)

func TestClient_Ingest(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestClient(t)
	defer cleanup()

	entries := []ragclient.LogEntry{
		{ID: "1", Body: "error: connection failed", Service: "api", Severity: "ERROR"},
		{ID: "2", Body: "error: timeout", Service: "api", Severity: "ERROR"},
	}

	result, err := client.Ingest(ctx, entries)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.IngestedCount, 0)
}

func TestClient_Search(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestClient(t)
	defer cleanup()

	// First ingest some data
	_, err := client.Ingest(ctx, []ragclient.LogEntry{
		{ID: "1", Body: "database connection error", Service: "db", Severity: "ERROR"},
	})
	require.NoError(t, err)

	results, err := client.Search(ctx, "database error", ragclient.SearchOptions{
		Limit:     10,
		TimeRange: time.Hour,
	})
	require.NoError(t, err)
	assert.NotNil(t, results)
}

func TestClient_GetContext(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestClient(t)
	defer cleanup()

	now := time.Now()
	results, err := client.GetContext(ctx, "api", now.Add(-time.Hour), now, 10)
	require.NoError(t, err)
	assert.NotNil(t, results)
}

func TestClient_Close(t *testing.T) {
	client, cleanup := setupTestClient(t)
	assert.NoError(t, client.Close())
	cleanup()
}

// mockEmbedder implements embedder.Interface for testing.
type mockEmbedder struct{}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vecs[i] = []float32{float32(i) * 0.1, float32(i) * 0.2, float32(i) * 0.3}
	}
	return vecs, nil
}

// inMemoryVectorStore implements vectorstore.VectorStore for testing.
type inMemoryVectorStore struct {
	records []vectorstore.Record
}

func (m *inMemoryVectorStore) Insert(ctx context.Context, records []vectorstore.Record) error {
	m.records = append(m.records, records...)
	return nil
}

func (m *inMemoryVectorStore) Search(ctx context.Context, vector []float32, opts vectorstore.SearchOptions) ([]vectorstore.SearchResult, error) {
	results := make([]vectorstore.SearchResult, len(m.records))
	for i, r := range m.records {
		results[i] = vectorstore.SearchResult{
			ID:        r.ID,
			Body:      r.Body,
			Service:   r.Service,
			Severity:  r.Severity,
			Timestamp: r.Timestamp,
			Score:     0.9,
		}
	}
	return results, nil
}

func (m *inMemoryVectorStore) Delete(ctx context.Context, ids []string) error { return nil }
func (m *inMemoryVectorStore) Close() error                                   { return nil }

func setupTestClient(t *testing.T) (ragclient.Client, func()) {
	t.Helper()

	emb := &mockEmbedder{}
	vs := &inMemoryVectorStore{}
	ragSvc := rag.NewService(emb, vs)
	ragServer := rag.NewGRPCServer(ragSvc)

	grpcServer := grpc.NewServer()
	ragpb.RegisterServiceServer(grpcServer, ragServer)

	listener := bufconn.Listen(1024 * 1024)
	go func() {
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Logf("gRPC server error: %v", err)
		}
	}()

	client, err := ragclient.New("passthrough:///bufnet",
		ragclient.WithGRPCDialOpts(
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return listener.Dial()
			}),
			grpc.WithInsecure(),
		),
	)
	require.NoError(t, err)

	cleanup := func() {
		_ = client.Close()
		grpcServer.GracefulStop()
	}

	return client, cleanup
}
