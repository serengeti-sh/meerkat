package server_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	analyzerMocks "github.com/serengeti-sh/meerkat/internal/analyzer/mocks"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/server"
	"github.com/serengeti-sh/meerkat/internal/tool"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "..", "..")
}

func TestNewLogsClient(t *testing.T) {
	t.Run("vectors disabled", func(t *testing.T) {
		cfg := &config.Config{
			Vectors: config.VectorsConfig{
				Enabled: false,
			},
		}
		client, err := server.ExportNewLogsClient(cfg)
		require.NoError(t, err)
		assert.Nil(t, client)
	})

	t.Run("vectors enabled but no address", func(t *testing.T) {
		cfg := &config.Config{
			Vectors: config.VectorsConfig{
				Enabled: true,
				Address: "",
			},
		}
		client, err := server.ExportNewLogsClient(cfg)
		require.NoError(t, err)
		assert.Nil(t, client)
	})

	t.Run("vectors enabled with address", func(t *testing.T) {
		cfg := &config.Config{
			Vectors: config.VectorsConfig{
				Enabled: true,
				Address: "localhost:50051",
			},
		}
		client, err := server.ExportNewLogsClient(cfg)
		require.NoError(t, err)
		assert.NotNil(t, client)
	})
}

func TestBuildDatasourceRefs(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		cfg := &config.Config{}
		refsFn := server.ExportBuildDatasourceRefs(cfg)
		refs := refsFn()
		assert.Empty(t, refs)
	})

	t.Run("with prometheus", func(t *testing.T) {
		cfg := &config.Config{
			Tools: config.ToolConfig{
				Prometheus: []config.PrometheusToolConfig{
					{Name: "vm-1", URL: "http://localhost:8428"},
					{Name: "vm-2", URL: "http://localhost:8429"},
				},
			},
		}
		refsFn := server.ExportBuildDatasourceRefs(cfg)
		refs := refsFn()
		require.Len(t, refs, 2)
		assert.Equal(t, "vm-1", refs[0].Name)
		assert.Equal(t, "prometheus", refs[0].Type)
		assert.Equal(t, "vm-2", refs[1].Name)
		assert.Equal(t, "prometheus", refs[1].Type)
	})

	t.Run("with victoria logs", func(t *testing.T) {
		cfg := &config.Config{
			Tools: config.ToolConfig{
				VictoriaLogs: []config.VictoriaLogsToolConfig{
					{Name: "vl-1", URL: "http://localhost:9428"},
				},
			},
		}
		refsFn := server.ExportBuildDatasourceRefs(cfg)
		refs := refsFn()
		require.Len(t, refs, 1)
		assert.Equal(t, "vl-1", refs[0].Name)
		assert.Equal(t, "victoria-logs", refs[0].Type)
	})

	t.Run("with loki", func(t *testing.T) {
		cfg := &config.Config{
			Tools: config.ToolConfig{
				Loki: []config.LokiToolConfig{
					{Name: "loki-1", URL: "http://localhost:3100"},
				},
			},
		}
		refsFn := server.ExportBuildDatasourceRefs(cfg)
		refs := refsFn()
		require.Len(t, refs, 1)
		assert.Equal(t, "loki-1", refs[0].Name)
		assert.Equal(t, "loki", refs[0].Type)
	})

	t.Run("mixed datasources", func(t *testing.T) {
		cfg := &config.Config{
			Tools: config.ToolConfig{
				Prometheus: []config.PrometheusToolConfig{
					{Name: "prom", URL: "http://prom:9090"},
				},
				VictoriaLogs: []config.VictoriaLogsToolConfig{
					{Name: "vl", URL: "http://vl:9428"},
				},
				Loki: []config.LokiToolConfig{
					{Name: "loki", URL: "http://loki:3100"},
				},
			},
		}
		refsFn := server.ExportBuildDatasourceRefs(cfg)
		refs := refsFn()
		require.Len(t, refs, 3)
		assert.Equal(t, "prom", refs[0].Name)
		assert.Equal(t, "vl", refs[1].Name)
		assert.Equal(t, "loki", refs[2].Name)
	})
}

func TestBuildToolRegistry(t *testing.T) {
	t.Run("empty tools", func(t *testing.T) {
		cfg := &config.Config{}
		registry, err := server.ExportBuildToolRegistry(cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, registry)
		tools := registry.All()
		assert.Empty(t, tools)
	})

	t.Run("with prometheus tool", func(t *testing.T) {
		root := repoRoot(t)
		cfg := &config.Config{
			Tools: config.ToolConfig{
				Prometheus: []config.PrometheusToolConfig{
					{Name: "test-prom", URL: "http://localhost:8428"},
				},
				PrometheusParamSchemaFile: filepath.Join(root, "internal", "tool", "schemas", "prometheus.json"),
			},
		}
		registry, err := server.ExportBuildToolRegistry(cfg, nil)
		require.NoError(t, err)
		tools := registry.All()
		require.Len(t, tools, 1)
		assert.Equal(t, "test-prom", tools[0].Name())
	})

	t.Run("with victoria logs tool", func(t *testing.T) {
		root := repoRoot(t)
		cfg := &config.Config{
			Tools: config.ToolConfig{
				VictoriaLogs: []config.VictoriaLogsToolConfig{
					{Name: "test-vl", URL: "http://localhost:9428"},
				},
				VictoriaLogsParamSchemaFile: filepath.Join(root, "internal", "tool", "schemas", "victorialogs.json"),
			},
		}
		registry, err := server.ExportBuildToolRegistry(cfg, nil)
		require.NoError(t, err)
		tools := registry.All()
		require.Len(t, tools, 1)
		assert.Equal(t, "test-vl", tools[0].Name())
	})

	t.Run("with loki tool", func(t *testing.T) {
		root := repoRoot(t)
		cfg := &config.Config{
			Tools: config.ToolConfig{
				Loki: []config.LokiToolConfig{
					{Name: "test-loki", URL: "http://localhost:3100"},
				},
				LokiParamSchemaFile: filepath.Join(root, "internal", "tool", "schemas", "loki.json"),
			},
		}
		registry, err := server.ExportBuildToolRegistry(cfg, nil)
		require.NoError(t, err)
		tools := registry.All()
		require.Len(t, tools, 1)
		assert.Equal(t, "test-loki", tools[0].Name())
	})

	t.Run("with logs client", func(t *testing.T) {
		cfg := &config.Config{}
		registry, err := server.ExportBuildToolRegistry(cfg, nil)
		require.NoError(t, err)
		assert.NotNil(t, registry)
	})
}

func TestBuildAnalyzerService(t *testing.T) {
	t.Run("successful build", func(t *testing.T) {
		tmpDir := t.TempDir()
		promptFile := filepath.Join(tmpDir, "prompt.txt")
		err := os.WriteFile(promptFile, []byte("You are a test agent."), 0644)
		require.NoError(t, err)

		cfg := &config.Config{
			Analyzer: config.AnalyzerConfig{
				SystemPromptFile: promptFile,
				SkillsFile:       "/dev/null",
				MaxIterations:    5,
			},
		}

		provider := analyzerMocks.NewLLMProviderMock(t)
		registry := tool.NewRegistry()

		svc, err := server.ExportBuildAnalyzerService(provider, registry, cfg)
		require.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("missing system prompt file", func(t *testing.T) {
		cfg := &config.Config{
			Analyzer: config.AnalyzerConfig{
				SystemPromptFile: "/nonexistent/prompt.txt",
			},
		}

		provider := analyzerMocks.NewLLMProviderMock(t)
		registry := tool.NewRegistry()

		_, err := server.ExportBuildAnalyzerService(provider, registry, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load system prompt")
	})
}
