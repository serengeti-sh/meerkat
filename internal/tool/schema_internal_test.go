package tool_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/tool"
)

func TestCompileSchema(t *testing.T) {
	t.Run("valid schema", func(t *testing.T) {
		tmpDir := t.TempDir()
		schemaFile := filepath.Join(tmpDir, "test.json")
		schema := `{
			"type": "object",
			"properties": {
				"query": {"type": "string"}
			}
		}`
		err := os.WriteFile(schemaFile, []byte(schema), 0644)
		require.NoError(t, err)

		compiled, raw, err := tool.ExportCompileSchema(schemaFile)
		require.NoError(t, err)
		assert.NotNil(t, compiled)
		assert.NotNil(t, raw)
	})

	t.Run("missing file", func(t *testing.T) {
		_, _, err := tool.ExportCompileSchema("/nonexistent/schema.json")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read schema file")
	})

	t.Run("invalid json", func(t *testing.T) {
		tmpDir := t.TempDir()
		schemaFile := filepath.Join(tmpDir, "invalid.json")
		err := os.WriteFile(schemaFile, []byte(`{invalid`), 0644)
		require.NoError(t, err)

		_, _, err = tool.ExportCompileSchema(schemaFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse schema file")
	})
}

func TestValidateArgs(t *testing.T) {
	t.Run("nil schema", func(t *testing.T) {
		err := tool.ExportValidateArgs(nil, json.RawMessage(`{"query": "test"}`))
		assert.NoError(t, err)
	})

	t.Run("valid args", func(t *testing.T) {
		tmpDir := t.TempDir()
		schemaFile := filepath.Join(tmpDir, "test.json")
		schema := `{
			"type": "object",
			"properties": {
				"query": {"type": "string"}
			}
		}`
		err := os.WriteFile(schemaFile, []byte(schema), 0644)
		require.NoError(t, err)

		compiled, _, err := tool.ExportCompileSchema(schemaFile)
		require.NoError(t, err)

		err = tool.ExportValidateArgs(compiled, json.RawMessage(`{"query": "test"}`))
		assert.NoError(t, err)
	})

	t.Run("invalid json", func(t *testing.T) {
		tmpDir := t.TempDir()
		schemaFile := filepath.Join(tmpDir, "test.json")
		schema := `{
			"type": "object",
			"properties": {
				"query": {"type": "string"}
			}
		}`
		err := os.WriteFile(schemaFile, []byte(schema), 0644)
		require.NoError(t, err)

		compiled, _, err := tool.ExportCompileSchema(schemaFile)
		require.NoError(t, err)

		err = tool.ExportValidateArgs(compiled, json.RawMessage(`{invalid`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid JSON arguments")
	})
}

func TestFormatParam(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{"string", "hello", "hello"},
		{"float64", 12.34, "12.34"},
		{"float64 integer", 42.0, "42"},
		{"int", 42, "42"},
		{"int64", int64(42), "42"},
		{"bool", true, "true"},
		{"slice", []string{"a", "b"}, `[a b]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tool.ExportFormatParam(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseTime(t *testing.T) {
	t.Run("rfc3339", func(t *testing.T) {
		result, err := tool.ExportParseTime("2024-01-15T10:30:00Z")
		require.NoError(t, err)
		assert.Equal(t, 2024, result.Year())
		assert.Equal(t, time.January, result.Month())
		assert.Equal(t, 15, result.Day())
	})

	t.Run("unix timestamp", func(t *testing.T) {
		result, err := tool.ExportParseTime("1705315800")
		require.NoError(t, err)
		assert.WithinDuration(t, time.Unix(1705315800, 0), result, time.Second)
	})

	t.Run("relative duration", func(t *testing.T) {
		before := time.Now()
		result, err := tool.ExportParseTime("1h")
		require.NoError(t, err)
		after := time.Now()
		assert.True(t, result.After(before.Add(-2*time.Hour)))
		assert.True(t, result.Before(after.Add(-time.Hour)))
	})

	t.Run("invalid format", func(t *testing.T) {
		_, err := tool.ExportParseTime("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid time format")
	})
}

func TestArgsToQueryParams(t *testing.T) {
	t.Run("with defaults", func(t *testing.T) {
		tmpDir := t.TempDir()
		schemaFile := filepath.Join(tmpDir, "test.json")
		schema := `{
			"type": "object",
			"properties": {
				"query": {"type": "string"},
				"start": {"type": "string"}
			}
		}`
		err := os.WriteFile(schemaFile, []byte(schema), 0644)
		require.NoError(t, err)

		compiled, _, err := tool.ExportCompileSchema(schemaFile)
		require.NoError(t, err)

		defaults := url.Values{"start": []string{"-1h"}}
		params, err := tool.ExportArgsToQueryParams(compiled, json.RawMessage(`{"query": "up"}`), defaults)
		require.NoError(t, err)
		assert.Equal(t, "up", params.Get("query"))
		assert.Equal(t, "-1h", params.Get("start"))
	})
}
