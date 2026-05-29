package report_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/serengeti-sh/meerkat/internal/ent"
	"github.com/serengeti-sh/meerkat/internal/inspect"
	"github.com/serengeti-sh/meerkat/internal/report"
)

func setupTestDB(t *testing.T) (*ent.Client, func()) {
	ctx := context.Background()

	c, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase("meerkat_test"),
		postgres.WithUsername("meerkat"),
		postgres.WithPassword("meerkat"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	client, err := ent.Open("postgres", dsn)
	require.NoError(t, err)

	err = inspect.Migrate(ctx, client)
	require.NoError(t, err)

	cleanup := func() {
		_ = client.Close()
		_ = c.Terminate(ctx)
	}

	return client, cleanup
}

func TestEntReportRepository_Create(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	r := &report.Report{
		ID:          "rpt-1",
		Trigger:     report.TriggerManual,
		TriggerID:   "t-1",
		Status:      report.StatusQueued,
		Severity:    report.SeverityInfo,
		Summary:     "test summary",
		Detail:      "test detail",
		Query:       "check errors",
		Datasources: []string{"prometheus", "loki"},
		Iterations:  3,
		CreatedAt:   time.Now(),
	}

	err := repo.Create(ctx, r)
	require.NoError(t, err)

	// Verify by fetching
	fetched, err := repo.GetByID(ctx, "rpt-1")
	require.NoError(t, err)
	assert.Equal(t, r.ID, fetched.ID)
	assert.Equal(t, r.Trigger, fetched.Trigger)
	assert.Equal(t, r.TriggerID, fetched.TriggerID)
	assert.Equal(t, r.Status, fetched.Status)
	assert.Equal(t, r.Severity, fetched.Severity)
	assert.Equal(t, r.Summary, fetched.Summary)
	assert.Equal(t, r.Detail, fetched.Detail)
	assert.Equal(t, r.Query, fetched.Query)
	assert.Equal(t, r.Datasources, fetched.Datasources)
	assert.Equal(t, r.Iterations, fetched.Iterations)
}

func TestEntReportRepository_Create_DuplicateID(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	r := &report.Report{
		ID:        "rpt-1",
		Trigger:   report.TriggerManual,
		TriggerID: "t-1",
		Status:    report.StatusQueued,
		Severity:  report.SeverityInfo,
	}

	err := repo.Create(ctx, r)
	require.NoError(t, err)

	// Creating another report with same ID should fail
	err = repo.Create(ctx, r)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create report")
}

func TestEntReportRepository_Update(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	r := &report.Report{
		ID:         "rpt-1",
		Trigger:    report.TriggerManual,
		TriggerID:  "t-1",
		Status:     report.StatusQueued,
		Severity:   report.SeverityInfo,
		Summary:    "initial",
		Iterations: 0,
	}

	err := repo.Create(ctx, r)
	require.NoError(t, err)

	// Update the report
	r.Status = report.StatusCompleted
	r.Severity = report.SeverityCritical
	r.Summary = "updated summary"
	r.Iterations = 5

	err = repo.Update(ctx, r)
	require.NoError(t, err)

	// Verify update
	fetched, err := repo.GetByID(ctx, "rpt-1")
	require.NoError(t, err)
	assert.Equal(t, report.StatusCompleted, fetched.Status)
	assert.Equal(t, report.SeverityCritical, fetched.Severity)
	assert.Equal(t, "updated summary", fetched.Summary)
	assert.Equal(t, 5, fetched.Iterations)
}

func TestEntReportRepository_Update_NotFound(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	r := &report.Report{
		ID:       "nonexistent",
		Status:   report.StatusCompleted,
		Severity: report.SeverityInfo,
	}

	err := repo.Update(ctx, r)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "update report")
}

func TestEntReportRepository_GetByID_NotFound(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	// Get non-existent report
	_, err := repo.GetByID(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get report by id")
}

func TestEntReportRepository_List(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	// Empty list
	reports, err := repo.List(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, reports)

	// Create reports
	for i := 0; i < 3; i++ {
		r := &report.Report{
			ID:        fmt.Sprintf("rpt-%d", i),
			Trigger:   report.TriggerManual,
			TriggerID: fmt.Sprintf("t-%d", i),
			Status:    report.StatusQueued,
			Severity:  report.SeverityInfo,
			Summary:   fmt.Sprintf("summary %d", i),
		}
		err := repo.Create(ctx, r)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different create times
	}

	// List all
	reports, err = repo.List(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, reports, 3)

	// Should be ordered by create time desc (newest first)
	assert.Equal(t, "rpt-2", reports[0].ID)
	assert.Equal(t, "rpt-1", reports[1].ID)
	assert.Equal(t, "rpt-0", reports[2].ID)

	// List with limit
	reports, err = repo.List(ctx, 2)
	require.NoError(t, err)
	assert.Len(t, reports, 2)

	// List with default limit (limit <= 0)
	reports, err = repo.List(ctx, 0)
	require.NoError(t, err)
	assert.Len(t, reports, 3) // default is 50, but we only have 3
}

func TestEntReportRepository_FindActiveByQuery(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	query := "check errors"
	trigger := string(report.TriggerManual)

	// No active reports
	found, err := repo.FindActiveByQuery(ctx, trigger, query, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Nil(t, found)

	// Create an active report
	r := &report.Report{
		ID:        "rpt-1",
		Trigger:   report.TriggerManual,
		TriggerID: "t-1",
		Status:    report.StatusQueued,
		Severity:  report.SeverityInfo,
		Query:     query,
	}
	err = repo.Create(ctx, r)
	require.NoError(t, err)

	// Should find active report
	found, err = repo.FindActiveByQuery(ctx, trigger, query, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, "rpt-1", found.ID)

	// Different query should not find
	found, err = repo.FindActiveByQuery(ctx, trigger, "different query", time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Nil(t, found)

	// Different trigger should not find
	found, err = repo.FindActiveByQuery(ctx, string(report.TriggerWebhook), query, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Nil(t, found)

	// Outside time window should not find
	found, err = repo.FindActiveByQuery(ctx, trigger, query, time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestEntReportRepository_FindActiveByQuery_MultipleStatuses(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	query := "check errors"
	trigger := string(report.TriggerManual)

	// Test with different active statuses
	statuses := []report.Status{
		report.StatusQueued,
		report.StatusPending,
		report.StatusRunning,
	}

	for i, status := range statuses {
		r := &report.Report{
			ID:        fmt.Sprintf("rpt-%d", i),
			Trigger:   report.TriggerManual,
			TriggerID: fmt.Sprintf("t-%d", i),
			Status:    status,
			Severity:  report.SeverityInfo,
			Query:     query,
		}
		err := repo.Create(ctx, r)
		require.NoError(t, err)
	}

	// Should find all active statuses, returning the most recent
	found, err := repo.FindActiveByQuery(ctx, trigger, query, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, report.StatusRunning, found.Status)

	// Completed reports should not be found
	r := &report.Report{
		ID:        "rpt-completed",
		Trigger:   report.TriggerManual,
		TriggerID: "t-completed",
		Status:    report.StatusCompleted,
		Severity:  report.SeverityInfo,
		Query:     query,
	}
	err = repo.Create(ctx, r)
	require.NoError(t, err)

	// Should still return running one (most recent active)
	found, err = repo.FindActiveByQuery(ctx, trigger, query, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, report.StatusRunning, found.Status)
}

func TestEntReportRepository_FindActiveByQuery_Error(t *testing.T) {
	client, cleanup := setupTestDB(t)
	defer cleanup()

	repo := report.NewEntReportRepository(client)
	ctx := context.Background()

	// Close client to simulate error
	client.Close()

	_, err := repo.FindActiveByQuery(ctx, "manual", "query", time.Now().Add(-time.Hour))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "find active report")
}
