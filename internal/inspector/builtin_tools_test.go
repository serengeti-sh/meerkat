package inspector_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/inspector/internal/datasource"
	dsMocks "github.com/mandacode-labs/inspector/internal/datasource/mocks"
	"github.com/mandacode-labs/inspector/internal/inspector"
)

func makeRegistryWithMetrics(t *testing.T, name string, querier datasource.MetricsQuerier) *datasource.Registry {
	t.Helper()
	p := dsMocks.NewProviderMock(t)
	p.EXPECT().Name().Return(name).Maybe()
	p.EXPECT().Type().Return(datasource.TypePrometheus).Maybe()
	p.EXPECT().MetricsQuerier().Return(querier, true).Maybe()
	p.EXPECT().LogsQuerier().Return(nil, false).Maybe()
	p.EXPECT().TestConnection(mock.Anything).Return(nil).Maybe()
	return datasource.NewRegistry([]datasource.Provider{p})
}

func makeRegistryWithLogs(t *testing.T, name string, querier datasource.LogsQuerier) *datasource.Registry {
	t.Helper()
	p := dsMocks.NewProviderMock(t)
	p.EXPECT().Name().Return(name).Maybe()
	p.EXPECT().Type().Return(datasource.TypeVictoriaLogs).Maybe()
	p.EXPECT().MetricsQuerier().Return(nil, false).Maybe()
	p.EXPECT().LogsQuerier().Return(querier, true).Maybe()
	p.EXPECT().TestConnection(mock.Anything).Return(nil).Maybe()
	return datasource.NewRegistry([]datasource.Provider{p})
}

func TestQueryMetricsTool_Success(t *testing.T) {
	querier := dsMocks.NewMetricsQuerierMock(t)
	querier.EXPECT().QueryMetrics(
		mock.Anything, "up",
	).Return([]datasource.TimeSeries{
		{Labels: map[string]string{"__name__": "up"}, Points: []datasource.DataPoint{{Timestamp: 1, Value: 1}}},
	}, nil)

	registry := makeRegistryWithMetrics(t, "vm", querier)
	tool := inspector.NewQueryMetricsTool(registry)

	args, _ := json.Marshal(map[string]string{
		"datasource_name": "vm",
		"query":           "up",
	})

	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)
	assert.Contains(t, result, `"value":1`)
}

func TestQueryMetricsTool_UnknownDatasource(t *testing.T) {
	registry := datasource.NewRegistry(nil)
	tool := inspector.NewQueryMetricsTool(registry)

	args, _ := json.Marshal(map[string]string{
		"datasource_name": "unknown",
		"query":           "up",
	})

	_, err := tool.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestQueryMetricsTool_UnsupportedDatasource(t *testing.T) {
	logsQuerier := dsMocks.NewLogsQuerierMock(t)
	registry := makeRegistryWithLogs(t, "vl", logsQuerier)
	tool := inspector.NewQueryMetricsTool(registry)

	args, _ := json.Marshal(map[string]string{
		"datasource_name": "vl",
		"query":           "up",
	})

	_, err := tool.Execute(context.Background(), args)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not support metrics")
}

func TestQueryLogsTool_Success(t *testing.T) {
	querier := dsMocks.NewLogsQuerierMock(t)
	querier.EXPECT().QueryLogs(
		mock.Anything, "level:error", 50,
	).Return([]datasource.LogEntry{
		{Timestamp: "2024-01-01", Message: "something broke", Level: "error"},
	}, nil)

	registry := makeRegistryWithLogs(t, "vl", querier)
	tool := inspector.NewQueryLogsTool(registry)

	args, _ := json.Marshal(map[string]any{
		"datasource_name": "vl",
		"query":           "level:error",
	})

	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)
	assert.Contains(t, result, "something broke")
}

func TestQueryLogsTool_CustomLimit(t *testing.T) {
	querier := dsMocks.NewLogsQuerierMock(t)
	querier.EXPECT().QueryLogs(
		mock.Anything, "level:error", 10,
	).Return(nil, nil)

	registry := makeRegistryWithLogs(t, "vl", querier)
	tool := inspector.NewQueryLogsTool(registry)

	args, _ := json.Marshal(map[string]any{
		"datasource_name": "vl",
		"query":           "level:error",
		"limit":           10,
	})

	result, err := tool.Execute(context.Background(), args)
	require.NoError(t, err)
	assert.Equal(t, "null", result)
}

func TestQueryMetricsTool_InvalidParams(t *testing.T) {
	registry := datasource.NewRegistry(nil)
	tool := inspector.NewQueryMetricsTool(registry)

	_, err := tool.Execute(context.Background(), json.RawMessage(`invalid`))
	require.Error(t, err)
}
