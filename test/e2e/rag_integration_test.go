package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/serengeti-sh/meerkat/pkg/ragpb"
)

// mockRAGServer implements a minimal RAG gRPC server for E2E testing.
type mockRAGServer struct {
	ragpb.UnimplementedServiceServer
	entries []*ragpb.LogEntry
}

func (m *mockRAGServer) Ingest(ctx context.Context, req *ragpb.IngestRequest) (*ragpb.IngestResponse, error) {
	m.entries = append(m.entries, req.Entries...)
	return &ragpb.IngestResponse{
		IngestedCount:     int32(len(req.Entries)),
		DeduplicatedCount: 0,
	}, nil
}

func (m *mockRAGServer) Search(ctx context.Context, req *ragpb.SearchRequest) (*ragpb.SearchResponse, error) {
	return &ragpb.SearchResponse{Results: m.toResults()}, nil
}

func (m *mockRAGServer) GetContext(ctx context.Context, req *ragpb.GetContextRequest) (*ragpb.GetContextResponse, error) {
	return &ragpb.GetContextResponse{Results: m.toResults()}, nil
}

func (m *mockRAGServer) toResults() []*ragpb.SearchResult {
	results := make([]*ragpb.SearchResult, len(m.entries))
	for i, e := range m.entries {
		results[i] = &ragpb.SearchResult{
			Id:        e.Id,
			Score:     0.95,
			Body:      e.Body,
			Service:   e.Service,
			Severity:  e.Severity,
			Timestamp: e.Timestamp,
		}
	}
	return results
}

// TestE2E_Webhook_WithRAGContext verifies that when a webhook is received
// and a RAG server is available, the analyzer receives log context in its prompt.
func TestE2E_Webhook_WithRAGContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx := context.Background()

	// 1. Start mock RAG gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ragPort := lis.Addr().(*net.TCPAddr).Port

	ragSvc := &mockRAGServer{}
	ragGRPC := grpc.NewServer()
	ragpb.RegisterServiceServer(ragGRPC, ragSvc)

	go func() {
		if err := ragGRPC.Serve(lis); err != nil {
			t.Logf("mock RAG server error: %v", err)
		}
	}()
	defer ragGRPC.GracefulStop()

	// 2. Pre-populate mock RAG with logs
	_, err = ragSvc.Ingest(ctx, &ragpb.IngestRequest{
		Entries: []*ragpb.LogEntry{
			{
				Id:        "log-1",
				Timestamp: timestamppb.New(time.Now().Add(-5 * time.Minute)),
				Service:   "payment-api",
				Severity:  "ERROR",
				Body:      "connection refused to payment database",
			},
			{
				Id:        "log-2",
				Timestamp: timestamppb.New(time.Now().Add(-3 * time.Minute)),
				Service:   "payment-api",
				Severity:  "ERROR",
				Body:      "timeout calling upstream payment processor",
			},
		},
	})
	require.NoError(t, err)

	// 3. Start e2e suite with RAG server configured
	suite := SetupSuiteWithRAG(t, ragPort)

	// 4. Send webhook referencing the service
	payload := map[string]any{
		"alert":   "PaymentErrorsHigh",
		"message": "service=payment-api error rate > 5%",
		"source":  "grafana",
		"data": map[string]any{
			"service": "payment-api",
			"rate":    "8.5%",
		},
	}
	resp, err := suite.Post("/v1/webhook", payload)
	require.NoError(t, err)
	assert.Equal(t, 202, resp.StatusCode)

	var createResult map[string]any
	require.NoError(t, suite.ReadJSON(resp, &createResult))
	reportID := createResult["id"].(string)

	// 5. Wait for analysis
	report, err := suite.WaitForReportStatus(reportID, "completed", 20*time.Second)
	require.NoError(t, err)

	assert.Equal(t, "completed", report["status"])
	assert.NotEmpty(t, report["summary"])
}

// SetupSuiteWithRAG creates an e2e suite with RAG_ADDRESS env var set.
func SetupSuiteWithRAG(t *testing.T, ragPort int) *Suite {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	suite := NewSuite(t)
	require.NoError(t, suite.StartWithRAG(ctx, ragPort), "Failed to start e2e test suite with RAG")
	t.Cleanup(func() {
		cancel()
		suite.Stop()
	})

	return suite
}
