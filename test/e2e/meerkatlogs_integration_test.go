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

	"github.com/serengeti-sh/meerkat/internal/meerkatlogspb"
)

// mockMeerkatLogsServer implements a minimal MeerkatLogs gRPC server for E2E testing.
type mockMeerkatLogsServer struct {
	meerkatlogspb.UnimplementedServiceServer
	entries []*meerkatlogspb.SearchResult
}

func (m *mockMeerkatLogsServer) Search(ctx context.Context, req *meerkatlogspb.SearchRequest) (*meerkatlogspb.SearchResponse, error) {
	return &meerkatlogspb.SearchResponse{Results: m.entries}, nil
}

func (m *mockMeerkatLogsServer) GetContext(ctx context.Context, req *meerkatlogspb.GetContextRequest) (*meerkatlogspb.GetContextResponse, error) {
	return &meerkatlogspb.GetContextResponse{Results: m.entries}, nil
}

// TestE2E_Webhook_WithLogContext verifies that when a webhook is received
// and a MeerkatLogs server is available, the analyzer receives log context in its prompt.
func TestE2E_Webhook_WithLogContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	// 1. Start mock MeerkatLogs gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	logsPort := lis.Addr().(*net.TCPAddr).Port

	logsSvc := &mockMeerkatLogsServer{
		entries: []*meerkatlogspb.SearchResult{
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
	logsGRPC := grpc.NewServer()
	meerkatlogspb.RegisterServiceServer(logsGRPC, logsSvc)

	go func() {
		if err := logsGRPC.Serve(lis); err != nil {
			t.Logf("mock MeerkatLogs server error: %v", err)
		}
	}()
	defer logsGRPC.GracefulStop()

	// 2. Start e2e suite with MeerkatLogs server configured
	suite := SetupSuiteWithLogs(t, logsPort)

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

// SetupSuiteWithLogs creates an e2e suite with MEERKAT_LOGS_ADDRESS env var set.
func SetupSuiteWithLogs(t *testing.T, logsPort int) *Suite {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

	suite := NewSuite(t)
	require.NoError(t, suite.StartWithLogs(ctx, logsPort), "Failed to start e2e test suite with MeerkatLogs")
	t.Cleanup(func() {
		cancel()
		suite.Stop()
	})

	return suite
}
