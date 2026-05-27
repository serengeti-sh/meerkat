package analyzer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	analyzerMocks "github.com/serengeti-sh/meerkat/internal/analyzer/mocks"
	toolMocks "github.com/serengeti-sh/meerkat/internal/tool/mocks"
)

func TestService_Analyze_SingleResponse(t *testing.T) {
	provider := analyzerMocks.NewLLMProviderMock(t)
	registry := analyzer.NewToolRegistry()
	svc := analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations: 5,
		SystemPrompt:  "",
	})

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
	tool := toolMocks.NewInterfaceMock(t)

	// Registration + Defs
	tool.EXPECT().Name().Return("query_metrics").Twice()
	tool.EXPECT().Description().Return("query metrics")
	tool.EXPECT().Parameters().Return(json.RawMessage(`{"type":"object"}`))

	registry := analyzer.NewToolRegistry(tool)
	svc := analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations:       10,
		MaxToolResultChars:  30000,
		SummarizeOnOverflow: true,
	})

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
	tool := toolMocks.NewInterfaceMock(t)

	// Registration + Defs
	tool.EXPECT().Name().Return("query_metrics").Twice()
	tool.EXPECT().Description().Return("query metrics")
	tool.EXPECT().Parameters().Return(json.RawMessage(`{"type":"object"}`))

	registry := analyzer.NewToolRegistry(tool)
	svc := analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations: 2,
	})

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

func TestService_Analyze_ToolResultTruncation(t *testing.T) {
	provider := analyzerMocks.NewLLMProviderMock(t)
	tool := toolMocks.NewInterfaceMock(t)

	tool.EXPECT().Name().Return("query_metrics").Twice()
	tool.EXPECT().Description().Return("query metrics")
	tool.EXPECT().Parameters().Return(json.RawMessage(`{"type":"object"}`))

	registry := analyzer.NewToolRegistry(tool)
	svc := analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations:      5,
		MaxToolResultChars: 100, // small limit for testing
	})

	// Tool returns a very long result
	longResult := strings.Repeat("x", 500)
	tool.EXPECT().Execute(mock.Anything, mock.Anything).Return(longResult, nil)

	completeCount := 0
	provider.EXPECT().Complete(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, req *analyzer.CompletionRequest) (*analyzer.CompletionResponse, error) {
			completeCount++
			if completeCount == 1 {
				return &analyzer.CompletionResponse{
					ToolCalls: []analyzer.ToolCall{{
						ID: "tc-1", Name: "query_metrics", Arguments: json.RawMessage(`{}`),
					}},
					Stop: false,
				}, nil
			}
			// Verify the tool result in messages is truncated
			var lastMsg analyzer.Message
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if req.Messages[i].Role == analyzer.RoleTool {
					lastMsg = req.Messages[i]
					break
				}
			}
			assert.Contains(t, lastMsg.Content, "[TRUNCATED:")
			assert.LessOrEqual(t, len(lastMsg.Content), 200) // 100 chars + truncation marker

			return &analyzer.CompletionResponse{
				Content: `{"severity":"info","summary":"ok","detail":"done"}`,
				Stop:    true,
			}, nil
		},
	)

	result, err := svc.Analyze(context.Background(), &analyzer.AnalysisInput{
		Trigger: "manual", TriggerID: "test-trunc",
	})

	require.NoError(t, err)
	assert.Equal(t, analyzer.SeverityInfo, result.Severity)
}

func TestService_Analyze_ContextOverflowRecovery(t *testing.T) {
	provider := analyzerMocks.NewLLMProviderMock(t)
	tool := toolMocks.NewInterfaceMock(t)

	tool.EXPECT().Name().Return("query_metrics").Twice()
	tool.EXPECT().Description().Return("query metrics")
	tool.EXPECT().Parameters().Return(json.RawMessage(`{"type":"object"}`))

	registry := analyzer.NewToolRegistry(tool)
	svc := analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations:       10,
		SummarizeOnOverflow: true,
	})

	// Need 3+ tool calls to have enough exchanges for summarization to trim
	tool.EXPECT().Execute(mock.Anything, mock.Anything).Return("ok", nil).Times(3)

	completeCount := 0
	provider.EXPECT().Complete(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, req *analyzer.CompletionRequest) (*analyzer.CompletionResponse, error) {
			completeCount++
			switch {
			case completeCount <= 3:
				// First 3 calls: request tools to build up conversation
				return &analyzer.CompletionResponse{
					ToolCalls: []analyzer.ToolCall{{
						ID:        fmt.Sprintf("tc-%d", completeCount),
						Name:      "query_metrics",
						Arguments: json.RawMessage(`{}`),
					}},
					Stop: false,
				}, nil
			case completeCount == 4:
				// Fourth call: simulate context overflow
				return nil, fmt.Errorf("%w: prompt too long", analyzer.ErrContextOverflow)
			default:
				// After recovery: succeed
				return &analyzer.CompletionResponse{
					Content: `{"severity":"info","summary":"recovered","detail":"analysis completed after recovery"}`,
					Stop:    true,
				}, nil
			}
		},
	)

	result, err := svc.Analyze(context.Background(), &analyzer.AnalysisInput{
		Trigger:   "manual",
		TriggerID: "test-overflow",
	})

	require.NoError(t, err)
	assert.Equal(t, analyzer.SeverityInfo, result.Severity)
	assert.Equal(t, "recovered", result.Summary)
}

func TestService_Analyze_ContextOverflowUnrecoverable(t *testing.T) {
	provider := analyzerMocks.NewLLMProviderMock(t)
	registry := analyzer.NewToolRegistry()
	svc := analyzer.NewService(provider, registry, analyzer.ServiceConfig{
		MaxIterations:       5,
		SummarizeOnOverflow: true,
	})

	// Immediately fail with context overflow — only system+user messages, nothing to trim
	provider.EXPECT().Complete(mock.Anything, mock.Anything).Return(nil, fmt.Errorf("%w: prompt too long", analyzer.ErrContextOverflow))

	_, err := svc.Analyze(context.Background(), &analyzer.AnalysisInput{
		Trigger:   "manual",
		TriggerID: "test-unrecoverable",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "context overflow")
}

func TestToolRegistry(t *testing.T) {
	t.Run("get existing tool", func(t *testing.T) {
		tool := toolMocks.NewInterfaceMock(t)
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
		tool := toolMocks.NewInterfaceMock(t)
		tool.EXPECT().Name().Return("test-tool").Twice()
		tool.EXPECT().Description().Return("a test tool")
		tool.EXPECT().Parameters().Return(json.RawMessage(`{"type":"object"}`))
		reg := analyzer.NewToolRegistry(tool)
		defs := reg.Defs()
		assert.Len(t, defs, 1)
		assert.Equal(t, "test-tool", defs[0].Name)
	})
}
