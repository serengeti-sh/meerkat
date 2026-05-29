package tool

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSchemaFiles_ValidJSON(t *testing.T) {
	files := []string{
		"schemas/prometheus.json",
		"schemas/victorialogs.json",
	}

	for _, path := range files {
		t.Run(path, func(t *testing.T) {
			schema, params, err := compileSchema(path)
			require.NoError(t, err, "schema file should be valid JSON and compile successfully")
			assert.NotNil(t, schema, "compiled schema should not be nil")
			assert.NotNil(t, params, "raw schema bytes should not be nil")
			assert.NotEmpty(t, params, "raw schema bytes should not be empty")
		})
	}
}

func TestValidateArgs_Valid(t *testing.T) {
	schema, _, err := compileSchema("schemas/prometheus.json")
	require.NoError(t, err)

	tests := []struct {
		name string
		args string
	}{
		{
			name: "simple query",
			args: `{"query":"up"}`,
		},
		{
			name: "query with time",
			args: `{"query":"rate(http_requests_total[5m])","time":"2024-01-01T00:00:00Z"}`,
		},
		{
			name: "query with timeout",
			args: `{"query":"up","timeout":"30s"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArgs(schema, []byte(tt.args))
			assert.NoError(t, err)
		})
	}
}

func TestValidateArgs_Invalid(t *testing.T) {
	schema, _, err := compileSchema("schemas/prometheus.json")
	require.NoError(t, err)

	tests := []struct {
		name string
		args string
	}{
		{
			name: "missing required query",
			args: `{"timeout":"30s"}`,
		},
		{
			name: "invalid json",
			args: `{"query":}`,
		},
		{
			name: "wrong type for query",
			args: `{"query":123}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArgs(schema, []byte(tt.args))
			assert.Error(t, err)
		})
	}
}

func TestValidateArgs_VictoriaLogsSchema(t *testing.T) {
	schema, _, err := compileSchema("schemas/victorialogs.json")
	require.NoError(t, err)

	err = validateArgs(schema, []byte(`{"query":"_time:5m _stream:{k8s.namespace.name=\"default\"} error","limit":50}`))
	assert.NoError(t, err)
}
