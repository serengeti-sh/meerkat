package inspector

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractServiceFromAlert(t *testing.T) {
	tests := []struct {
		name    string
		alert   string
		message string
		data    json.RawMessage
		want    string
	}{
		{
			name:    "service= in message",
			alert:   "HighErrorRate",
			message: "service=api-server has high error rate",
			want:    "api-server",
		},
		{
			name:    "service: in alert",
			alert:   "service:db-primary CPU high",
			message: "CPU usage is above 90%",
			want:    "db-primary",
		},
		{
			name:    "quoted service value",
			alert:   `service="payment-gateway"`,
			message: "",
			want:    "payment-gateway",
		},
		{
			name:    "app label",
			alert:   "HighLatency",
			message: `app=auth-service latency p99 > 500ms`,
			want:    "auth-service",
		},
		{
			name:    "job label",
			alert:   "DiskFull",
			message: `job="node-exporter" disk usage > 90%`,
			want:    "node-exporter",
		},
		{
			name:    "namespace label",
			alert:   "OOMKilled",
			message: `namespace=production pod crashed`,
			want:    "production",
		},
		{
			name:    "service_name key",
			alert:   "Error",
			message: `service_name=order-service error rate high`,
			want:    "order-service",
		},
		{
			name:    "JSON data with service field",
			alert:   "Alert",
			message: "Something happened",
			data:    json.RawMessage(`{"service":"notification-svc","severity":"critical"}`),
			want:    "notification-svc",
		},
		{
			name:    "JSON data with nested labels",
			alert:   "Alert",
			message: "Something happened",
			data:    json.RawMessage(`{"labels":{"service":"billing-api","env":"prod"}}`),
			want:    "billing-api",
		},
		{
			name:    "Grafana commonLabels format",
			alert:   "Alert",
			message: "Something happened",
			data:    json.RawMessage(`{"commonLabels":{"app":"grafana","service":"dashboard-svc"}}`),
			want:    "dashboard-svc",
		},
		{
			name:    "heuristic fallback",
			alert:   "HighErrorRate",
			message: "The api-gateway service is experiencing high error rates",
			want:    "api-gateway",
		},
		{
			name:    "no service found",
			alert:   "HighLatency",
			message: "Latency is above threshold",
			want:    "",
		},
		{
			name:    "empty inputs",
			alert:   "",
			message: "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServiceFromAlert(tt.alert, tt.message, tt.data)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractFromJSON_MultipleKeys(t *testing.T) {
	// service_name should take precedence over namespace if both present
	data := json.RawMessage(`{"service_name":"primary-svc","namespace":"infra"}`)
	got := extractFromJSON(data)
	assert.Equal(t, "primary-svc", got)

	// app should be used if service not present
	data = json.RawMessage(`{"app":"mobile-api","region":"us-east-1"}`)
	got = extractFromJSON(data)
	assert.Equal(t, "mobile-api", got)

	// job should be used as fallback
	data = json.RawMessage(`{"job":"backup-cron","status":"failed"}`)
	got = extractFromJSON(data)
	assert.Equal(t, "backup-cron", got)
}

func TestExtractHeuristic(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"the redis service is down", "redis"},
		{"service api-gateway timeout", "api-gateway"},
		{"no keyword here", ""},
		{"service=explicit", ""}, // heuristic should not match key=value
		{"service:explicit", ""}, // heuristic should not match key:value
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got := extractHeuristic(tt.text)
			assert.Equal(t, tt.want, got)
		})
	}
}
