package inspector

import "encoding/json"

// InspectRequest is the input for a manual inspection.
type InspectRequest struct {
	MetricQuery string `json:"metric_query,omitempty"`
	LogQuery    string `json:"log_query,omitempty"`
	Query       string `json:"query,omitempty"` // natural language query
}

// WebhookPayload is the input for a webhook-triggered inspection.
type WebhookPayload struct {
	Source  string          `json:"source"` // grafana, custom
	Alert   string          `json:"alert"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}
