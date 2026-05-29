package vectorsclient_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/serengeti-sh/meerkat/internal/vectorspb"
	"github.com/serengeti-sh/meerkat/internal/vectors"
	"github.com/serengeti-sh/meerkat/internal/vectorsclient"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

func TestClient_Search(t *testing.T) {
	ctx := context.Background()
	client, cleanup := setupTestClient(t)
	defer cleanup()

	results, err := client.Search(ctx, "database error", vectorsclient.SearchOptions{
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

// mockEmbedder implements embed.Model for testing.
type mockEmbedder struct{}

func (m *mockEmbedder) HealthCheck(ctx context.Context) error { return nil }

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i := range texts {
		vecs[i] = []float32{float32(i) * 0.1, float32(i) * 0.2, float32(i) * 0.3}
	}
	return vecs, nil
}

// inMemoryVectorStore implements vectorstore.Store for testing.
type inMemoryVectorStore struct {
	records []vectorstore.Record
}

func (m *inMemoryVectorStore) Insert(ctx context.Context, records []vectorstore.Record) error {
	m.records = append(m.records, records...)
	return nil
}

func (m *inMemoryVectorStore) Search(ctx context.Context, v []float32, opts vectorstore.SearchOptions) ([]vectorstore.SearchResult, error) {
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
func (m *inMemoryVectorStore) Ping(ctx context.Context) error                 { return nil }
func (m *inMemoryVectorStore) Close() error                                   { return nil }

func setupTestClient(t *testing.T) (vectorsclient.Client, func()) {
	t.Helper()

	emb := &mockEmbedder{}
	vs := &inMemoryVectorStore{}
	vectorsSvc, err := vectors.NewService(emb, vs)
	require.NoError(t, err)
	vectorsServer, err := vectors.NewGRPCServer(vectorsSvc)
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	vectorspb.RegisterServiceServer(grpcServer, vectorsServer)

	listener := bufconn.Listen(1024 * 1024)
	go func() {
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Logf("gRPC server error: %v", err)
		}
	}()

	client, err := vectorsclient.New("passthrough:///bufnet",
		vectorsclient.WithGRPCDialOpts(
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return listener.Dial()
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		),
	)
	require.NoError(t, err)

	cleanup := func() {
		_ = client.Close()
		grpcServer.GracefulStop()
	}

	return client, cleanup
}
