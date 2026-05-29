package report

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReport_Fields(t *testing.T) {
	now := time.Now()
	r := &Report{
		ID:          "r-123",
		Trigger:     TriggerWebhook,
		TriggerID:   "webhook-456",
		Status:      StatusRunning,
		Severity:    SeverityWarning,
		Summary:     "something is wrong",
		Detail:      "detailed analysis here",
		Query:       "check errors",
		Datasources: []string{"prometheus", "loki"},
		Iterations:  5,
		CreatedAt:   now,
	}

	assert.Equal(t, "r-123", r.ID)
	assert.Equal(t, TriggerWebhook, r.Trigger)
	assert.Equal(t, "webhook-456", r.TriggerID)
	assert.Equal(t, StatusRunning, r.Status)
	assert.Equal(t, SeverityWarning, r.Severity)
	assert.Equal(t, "something is wrong", r.Summary)
	assert.Equal(t, "detailed analysis here", r.Detail)
	assert.Equal(t, "check errors", r.Query)
	assert.Equal(t, []string{"prometheus", "loki"}, r.Datasources)
	assert.Equal(t, 5, r.Iterations)
	assert.Equal(t, now, r.CreatedAt)
}

func TestReport_Clone(t *testing.T) {
	now := time.Now()
	original := &Report{
		ID:          "r-123",
		Status:      StatusQueued,
		Severity:    SeverityInfo,
		Summary:     "original",
		Datasources: []string{"prometheus"},
		Iterations:  3,
		CreatedAt:   now,
	}

	cloned := original.Clone()
	cloned.Status = StatusCompleted
	cloned.Severity = SeverityCritical
	cloned.Summary = "updated"

	// Original should be unchanged.
	assert.Equal(t, StatusQueued, original.Status)
	assert.Equal(t, SeverityInfo, original.Severity)
	assert.Equal(t, "original", original.Summary)

	// Clone should have new values.
	assert.Equal(t, "r-123", cloned.ID)
	assert.Equal(t, StatusCompleted, cloned.Status)
	assert.Equal(t, SeverityCritical, cloned.Severity)
	assert.Equal(t, "updated", cloned.Summary)
	assert.Equal(t, []string{"prometheus"}, cloned.Datasources)
	assert.Equal(t, 3, cloned.Iterations)
	assert.Equal(t, now, cloned.CreatedAt)
}

func TestReport_Clone_DatasourcesIsolation(t *testing.T) {
	original := &Report{
		Datasources: []string{"a", "b"},
	}

	cloned := original.Clone()
	// Mutate clone's datasource slice.
	cloned.Datasources[0] = "mutated"

	// Original should be unaffected.
	assert.Equal(t, []string{"a", "b"}, original.Datasources)
}
