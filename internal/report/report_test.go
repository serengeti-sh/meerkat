package report

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewReport_Default(t *testing.T) {
	r := NewReport()
	assert.NotNil(t, r)
	assert.Empty(t, r.ID())
	assert.Empty(t, r.Status())
	assert.Empty(t, r.Summary())
}

func TestNewReport_WithOptions(t *testing.T) {
	now := time.Now()
	r := NewReport(
		WithID("r-123"),
		WithTrigger(TriggerWebhook),
		WithTriggerID("webhook-456"),
		WithStatus(StatusRunning),
		WithSeverity(SeverityWarning),
		WithSummary("something is wrong"),
		WithDetail("detailed analysis here"),
		WithQuery("check errors"),
		WithDatasources([]string{"prometheus", "loki"}),
		WithIterations(5),
		WithCreatedAt(now),
	)

	assert.Equal(t, "r-123", r.ID())
	assert.Equal(t, TriggerWebhook, r.Trigger())
	assert.Equal(t, "webhook-456", r.TriggerID())
	assert.Equal(t, StatusRunning, r.Status())
	assert.Equal(t, SeverityWarning, r.Severity())
	assert.Equal(t, "something is wrong", r.Summary())
	assert.Equal(t, "detailed analysis here", r.Detail())
	assert.Equal(t, "check errors", r.Query())
	assert.Equal(t, []string{"prometheus", "loki"}, r.Datasources())
	assert.Equal(t, 5, r.Iterations())
	assert.Equal(t, now, r.CreatedAt())
}

func TestReport_Clone(t *testing.T) {
	now := time.Now()
	original := NewReport(
		WithID("r-123"),
		WithStatus(StatusQueued),
		WithSeverity(SeverityInfo),
		WithSummary("original"),
		WithDatasources([]string{"prometheus"}),
		WithIterations(3),
		WithCreatedAt(now),
	)

	cloned := original.Clone(
		WithStatus(StatusCompleted),
		WithSeverity(SeverityCritical),
		WithSummary("updated"),
	)

	// Original should be unchanged.
	assert.Equal(t, StatusQueued, original.Status())
	assert.Equal(t, SeverityInfo, original.Severity())
	assert.Equal(t, "original", original.Summary())

	// Clone should have new values.
	assert.Equal(t, "r-123", cloned.ID())
	assert.Equal(t, StatusCompleted, cloned.Status())
	assert.Equal(t, SeverityCritical, cloned.Severity())
	assert.Equal(t, "updated", cloned.Summary())
	assert.Equal(t, []string{"prometheus"}, cloned.Datasources())
	assert.Equal(t, 3, cloned.Iterations())
	assert.Equal(t, now, cloned.CreatedAt())
}

func TestReport_Clone_DatasourcesIsolation(t *testing.T) {
	original := NewReport(
		WithDatasources([]string{"a", "b"}),
	)

	cloned := original.Clone()
	// Mutate clone's datasource slice.
	cloned.Datasources()[0] = "mutated"

	// Original should be unaffected.
	assert.Equal(t, []string{"a", "b"}, original.Datasources())
}

func TestReport_Getters(t *testing.T) {
	r := NewReport(
		WithID("id-1"),
		WithTrigger(TriggerManual),
		WithTriggerID("tid-1"),
		WithStatus(StatusFailed),
		WithSeverity(SeverityInfo),
		WithSummary("sum"),
		WithDetail("det"),
		WithQuery("q"),
		WithDatasources([]string{"ds1"}),
		WithIterations(7),
	)

	assert.Equal(t, "id-1", r.ID())
	assert.Equal(t, TriggerManual, r.Trigger())
	assert.Equal(t, "tid-1", r.TriggerID())
	assert.Equal(t, StatusFailed, r.Status())
	assert.Equal(t, SeverityInfo, r.Severity())
	assert.Equal(t, "sum", r.Summary())
	assert.Equal(t, "det", r.Detail())
	assert.Equal(t, "q", r.Query())
	assert.Equal(t, []string{"ds1"}, r.Datasources())
	assert.Equal(t, 7, r.Iterations())
}
