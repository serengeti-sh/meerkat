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

	"github.com/serengeti-sh/meerkat/internal/vectorspb"
)

// mockVectorsServer implements a minimal Vectors gRPC server for E2E testing.
type mockVectorsServer struct {
	vectorspb.UnimplementedServiceServer
	entries []*vectorspb.SearchResult
}

func (m *mockVectorsServer) Search(ctx context.Context, req *vectorspb.SearchRequest) (*vectorspb.SearchResponse, error) {
	return &vectorspb.SearchResponse{Results: m.entries}, nil
}

func (m *mockVectorsServer) GetContext(ctx context.Context, req *vectorspb.GetContextRequest) (*vectorspb.GetContextResponse, error) {
	return &vectorspb.GetContextResponse{Results: m.entries}, nil
}

// TestE2E_Webhook_WithLogContext verifies that when a webhook is received
// and a Vectors server is available, the analyzer receives log context in its prompt.
func TestE2E_Webhook_WithLogContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// 1. Start mock Vectors gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	vectorsPort := lis.Addr().(*net.TCPAddr).Port

	vectorsSvc := &mockVectorsServer{
		entries: []*vectorspb.SearchResult{
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
	}
	vectorsGRPC := grpc.NewServer()
	vectorspb.RegisterServiceServer(vectorsGRPC, vectorsSvc)

	go func() {
		if err := vectorsGRPC.Serve(lis); err != nil {
			t.Logf("mock Vectors server error: %v", err)
		}
	}()
	defer vectorsGRPC.GracefulStop()

	// 2. Start e2e suite with Vectors server configured
	suite := SetupSuiteWithVectors(t, vectorsPort)

	// 3. Send webhook referencing the service
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

	// 4. Wait for analysis
	report, err := suite.WaitForReportStatus(reportID, "completed", 20*time.Second)
	require.NoError(t, err)

	assert.Equal(t, "completed", report["status"])
	assert.NotEmpty(t, report["summary"])
}

// SetupSuiteWithVectors creates an e2e suite with VECTORS_ADDRESS env var set.
func SetupSuiteWithVectors(t *testing.T, vectorsPort int) *Suite {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	suite := NewSuite(t)
	require.NoError(t, suite.StartWithVectors(ctx, vectorsPort), "Failed to start e2e test suite with Vectors")
	t.Cleanup(func() {
		cancel()
		suite.Stop()
	})

	return suite
}
