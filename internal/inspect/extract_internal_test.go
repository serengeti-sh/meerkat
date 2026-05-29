package inspect_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/serengeti-sh/meerkat/internal/inspect"
)

func TestExtractServiceFromAlert(t *testing.T) {
	tests := []struct {
		name     string
		alert    string
		message  string
		data     json.RawMessage
		expected string
	}{
		{
			name:     "empty",
			expected: "",
		},
		{
			name:     "from alert text",
			alert:    `service="my-api"`,
			expected: "my-api",
		},
		{
			name:     "from message text",
			message:  `app=frontend`,
			expected: "frontend",
		},
		{
			name:     "from JSON data",
			data:     json.RawMessage(`{"service": "backend"}`),
			expected: "backend",
		},
		{
			name:     "from nested JSON",
			data:     json.RawMessage(`{"labels": {"service": "database"}}`),
			expected: "database",
		},
		{
			name:     "alert takes precedence over message",
			alert:    `service="alert-svc"`,
			message:  `service="msg-svc"`,
			expected: "alert-svc",
		},
		{
			name:     "heuristic fallback",
			message:  "frontend service degraded",
			expected: "frontend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inspect.ExportExtractServiceFromAlert(tt.alert, tt.message, tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractFromText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{"empty", "", ""},
		{"service equals", `service="api"`, "api"},
		{"service colon", `service: "api"`, "api"},
		{"app equals", `app=frontend`, "frontend"},
		{"job equals", `job="worker"`, "worker"},
		{"namespace equals", `namespace=production`, "production"},
		{"no match", `hello world`, ""},
		{"null value", `service=null`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inspect.ExportExtractFromText(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		data     json.RawMessage
		expected string
	}{
		{"empty", json.RawMessage{}, ""},
		{"invalid json", json.RawMessage(`{invalid`), ""},
		{"direct key", json.RawMessage(`{"service": "api"}`), "api"},
		{"service_name", json.RawMessage(`{"service_name": "backend"}`), "backend"},
		{"app", json.RawMessage(`{"app": "frontend"}`), "frontend"},
		{"nested labels", json.RawMessage(`{"labels": {"service": "db"}}`), "db"},
		{"nested commonLabels", json.RawMessage(`{"commonLabels": {"app": "web"}}`), "web"},
		{"null value", json.RawMessage(`{"service": null}`), ""},
		{"empty string", json.RawMessage(`{"service": ""}`), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inspect.ExportExtractFromJSON(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractHeuristic(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{"empty", "", ""},
		{"no service word", "hello world", ""},
		{"X service pattern", "frontend service degraded", "frontend"},
		{"service X pattern", "service backend is down", "backend"},
		{"stop word before", "the service", ""},
		{"quoted after", `service="api"`, ""},
		{"colon after", `service: api`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := inspect.ExportExtractHeuristic(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}
