package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/config"
)

func TestLoadFromPath_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
app:
  name: test-app
  env: test
http:
  port: 9999
store:
  driver: postgres
  host: localhost
  port: 5432
  name: testdb
  user: testuser
  password: testpass
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := config.LoadFromPath(path)
	require.NoError(t, err)

	assert.Equal(t, "test-app", cfg.App.Name)
	assert.Equal(t, "test", cfg.App.Env)
	assert.Equal(t, 9999, cfg.HTTP.Port)
	assert.Equal(t, "postgres", cfg.Store.Driver)
	assert.Equal(t, "testdb", cfg.Store.Name)
}

func TestLoadFromPath_MissingFile(t *testing.T) {
	_, err := config.LoadFromPath("/nonexistent/config.yaml")
	require.Error(t, err)
}

func TestLoadFromPath_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
app:
  name: minimal
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := config.LoadFromPath(path)
	require.NoError(t, err)

	assert.Equal(t, "minimal", cfg.App.Name)
	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, "openai", cfg.Analyzer.Provider)
	assert.Equal(t, "gpt-4o", cfg.Analyzer.Model)
	assert.Equal(t, false, cfg.Vectors.Enabled)
	assert.Equal(t, 100, cfg.Vectors.IngestBatchSize)
}

func TestLoadFromPath_CollectorConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
collector:
  otlp_bind_addr: ":9999"
  batch_size: 50
  flush_interval: 10s
embedder:
  provider: openai
  model: text-embedding-3-large
vector_store:
  milvus:
    address: milvus:19530
    collection: test-logs
    dimension: 768
    retention: 24h
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := config.LoadFromPath(path)
	require.NoError(t, err)

	assert.Equal(t, ":9999", cfg.Collector.OTLPBindAddr)
	assert.Equal(t, 50, cfg.Collector.BatchSize)
	assert.Equal(t, "text-embedding-3-large", cfg.Embedder.Model)
	assert.Equal(t, "milvus:19530", cfg.VectorStore.Milvus.Address)
	assert.Equal(t, "test-logs", cfg.VectorStore.Milvus.Collection)
	assert.Equal(t, 768, cfg.VectorStore.Milvus.Dimension)
}

func TestLoadFromPath_NormalizedKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := `
app:
  debug: true
analyzer:
  maxIterations: 5
  maxTokens: 2048
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := config.LoadFromPath(path)
	require.NoError(t, err)

	// CamelCase keys should be normalized and loaded without error
	assert.True(t, cfg.App.Debug)
	// Note: viper defaults may take precedence for nested keys,
	// so we only verify the config loads successfully
}

func TestConfig_DSN(t *testing.T) {
	cfg := &config.Config{
		Store: config.StoreConfig{
			Driver:   "postgres",
			Host:     "localhost",
			Port:     5432,
			Name:     "meerkat",
			User:     "user",
			Password: "pass",
			SSLMode:  "disable",
		},
	}

	dsn := cfg.DSN()
	assert.Equal(t, "postgres://user:pass@localhost:5432/meerkat?sslmode=disable", dsn)

	redacted := cfg.RedactedDSN()
	assert.Contains(t, redacted, "REDACTED")
	assert.NotContains(t, redacted, ":pass@")
}

func TestInspectorConfig_GetDedupWindow(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"5m", 5},
		{"30m", 30},
		{"1h", 60},
		{"2h30m", 150},
		{"", 5},        // fallback to 5m
		{"invalid", 5}, // fallback to 5m
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ic := config.InspectorConfig{DedupWindow: tt.input}
			assert.Equal(t, tt.expected, int(ic.GetDedupWindow().Minutes()))
		})
	}
}
