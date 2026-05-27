package analyzer

// AnalysisInput is what goes into the agent loop.
type AnalysisInput struct {
	Trigger     string          `json:"trigger"` // manual, webhook, scheduled
	TriggerID   string          `json:"trigger_id"`
	Query       string          `json:"query"` // optional specific query
	Datasources []DatasourceRef `json:"datasources"`
	Context     string          `json:"context"` // additional context (e.g. webhook payload)
}

// DatasourceRef identifies a datasource for analysis.
type DatasourceRef struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
