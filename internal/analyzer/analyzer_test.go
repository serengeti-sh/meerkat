package analyzer_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/inspector/internal/analyzer"
	analyzerMocks "github.com/mandacode-labs/inspector/internal/analyzer/mocks"
)

func TestService_Analyze_SingleResponse(t *testing.T) {
	provider := analyzerMocks.NewLLMProviderMock(t)
	registry := analyzer.NewToolRegistry()
	svc := analyzer.NewService(provider, registry, 5, "")

	provider.EXPECT().Complete(mock.Anything, mock.Anything).Return(&analyzer.CompletionResponse{
		Content: `{"severity":"warning","summary":"error spike detected","detail":"error rate increased 300%"}`,
		Stop:    true,
	}, nil)

	result, err := svc.Analyze(context.Background(), &analyzer.AnalysisInput{
		Trigger:   "manual",
		TriggerID: "test-1",
		Datasources: []analyzer.DatasourceRef{
			{Name: "vm", Type: "victoria-metrics"},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, analyzer.SeverityWarning, result.Severity)
	assert.Equal(t, "error spike detected", result.Summary)
	assert.Equal(t, 1, result.Iterations)
}

func TestService_Analyze_ToolCalls(t *testing.T) {
	provider := analyzerMocks.NewLLMProviderMock(t)
	tool := analyzerMocks.NewToolMock(t)

	// Registration + Defs
	tool.EXPECT().Name().Return("query_metrics").Twice()
	tool.EXPECT().Description().Return("query metrics")
	tool.EXPECT().Parameters().Return(json.RawMessage(`{"type":"object"}`))

	registry := analyzer.NewToolRegistry(tool)
	svc := analyzer.NewService(provider, registry, 10, "")

	// Use RunAndReturn for Execute to handle multiple calls
	execCount := 0
	tool.EXPECT().Execute(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, args json.RawMessage) (string, error) {
			execCount++
			if execCount == 1 {
				return `{"status":"success","data":[{"metric":{},"values":[[1712600000,"0.5"]]}]}`, nil
			}
			return `{"_time":"2024-04-09T10:00:00Z","_msg":"connection refused"}`, nil
		},
	)

	// Use RunAndReturn for Complete to control sequence
	completeCount := 0
	provider.EXPECT().Complete(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, req *analyzer.CompletionRequest) (*analyzer.CompletionResponse, error) {
			completeCount++
			switch completeCount {
			case 1:
				return &analyzer.CompletionResponse{
					Content: "Let me check the error rate.",
					ToolCalls: []analyzer.ToolCall{{
						ID:        "tc-1",
						Name:      "query_metrics",
						Arguments: json.RawMessage(`{"datasource_url":"http://localhost:8428","query":"rate(http_errors_total[5m])"}`),
					}},
					Stop: false,
				}, nil
			case 2:
				return &analyzer.CompletionResponse{
					Content: "Now let me check the logs.",
					ToolCalls: []analyzer.ToolCall{{
						ID:        "tc-2",
						Name:      "query_metrics",
						Arguments: json.RawMessage(`{"datasource_url":"http://localhost:9428","query":"level:error","limit":10}`),
					}},
					Stop: false,
				}, nil
			case 3:
				return &analyzer.CompletionResponse{
					Content: `{"severity":"critical","summary":"database connection pool exhausted","detail":"error rate spiked due to DB pool exhaustion"}`,
					Stop:    true,
				}, nil
			default:
				t.Fatalf("unexpected Complete call #%d", completeCount)
				return nil, nil
			}
		},
	)

	result, err := svc.Analyze(context.Background(), &analyzer.AnalysisInput{
		Trigger:     "manual",
		TriggerID:   "test-2",
		Query:       "check for error spikes",
		Datasources: []analyzer.DatasourceRef{{Name: "vm", Type: "victoria-metrics"}},
	})

	require.NoError(t, err)
	assert.Equal(t, analyzer.SeverityCritical, result.Severity)
	assert.Equal(t, "database connection pool exhausted", result.Summary)
	assert.Equal(t, 3, result.Iterations)
}

func TestService_Analyze_MaxIterations(t *testing.T) {
	provider := analyzerMocks.NewLLMProviderMock(t)
	tool := analyzerMocks.NewToolMock(t)

	// Registration + Defs
	tool.EXPECT().Name().Return("query_metrics").Twice()
	tool.EXPECT().Description().Return("query metrics")
	tool.EXPECT().Parameters().Return(json.RawMessage(`{"type":"object"}`))

	registry := analyzer.NewToolRegistry(tool)
	svc := analyzer.NewService(provider, registry, 2, "")

	tool.EXPECT().Execute(mock.Anything, mock.Anything).Return("ok", nil).Twice()

	completeCount := 0
	provider.EXPECT().Complete(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, req *analyzer.CompletionRequest) (*analyzer.CompletionResponse, error) {
			completeCount++
			if completeCount <= 2 {
				return &analyzer.CompletionResponse{
					Content:   "checking...",
					ToolCalls: []analyzer.ToolCall{{ID: string(rune(completeCount)), Name: "query_metrics", Arguments: json.RawMessage(`{}`)}},
					Stop:      false,
				}, nil
			}
			return &analyzer.CompletionResponse{
				Content: `{"severity":"info","summary":"analysis complete","detail":"no issues found"}`,
				Stop:    true,
			}, nil
		},
	)

	result, err := svc.Analyze(context.Background(), &analyzer.AnalysisInput{
		Trigger: "manual", TriggerID: "test-3",
	})

	require.NoError(t, err)
	assert.Equal(t, analyzer.SeverityInfo, result.Severity)
}

func TestToolRegistry(t *testing.T) {
	t.Run("get existing tool", func(t *testing.T) {
		tool := analyzerMocks.NewToolMock(t)
		tool.EXPECT().Name().Return("test-tool")
		reg := analyzer.NewToolRegistry(tool)
		found, ok := reg.Get("test-tool")
		assert.True(t, ok)
		assert.NotNil(t, found)
	})

	t.Run("get missing tool", func(t *testing.T) {
		reg := analyzer.NewToolRegistry()
		_, ok := reg.Get("nonexistent")
		assert.False(t, ok)
	})

	t.Run("defs returns tool definitions", func(t *testing.T) {
		tool := analyzerMocks.NewToolMock(t)
		tool.EXPECT().Name().Return("test-tool").Twice()
		tool.EXPECT().Description().Return("a test tool")
		tool.EXPECT().Parameters().Return(json.RawMessage(`{"type":"object"}`))
		reg := analyzer.NewToolRegistry(tool)
		defs := reg.Defs()
		assert.Len(t, defs, 1)
		assert.Equal(t, "test-tool", defs[0].Name)
	})
}
