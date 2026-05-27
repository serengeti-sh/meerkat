package reporter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/reporter"
)

func TestService_Report_Success(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		err := json.NewDecoder(r.Body).Decode(&received)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := reporter.NewService(srv.URL, "warning", http.DefaultClient)
	err := svc.Report(context.Background(), &reporter.ReportData{
		ID:       "r-1",
		Severity: "critical",
		Summary:  "High CPU detected",
		Detail:   "CPU usage exceeded 90%",
	})
	require.NoError(t, err)

	assert.NotNil(t, received)
	assert.Contains(t, received["text"], "critical")
	assert.Contains(t, received["text"], "High CPU detected")
}

func TestService_Report_EmptyWebhookURL(t *testing.T) {
	svc := reporter.NewService("", "warning", http.DefaultClient)
	err := svc.Report(context.Background(), &reporter.ReportData{
		ID:       "r-1",
		Severity: "critical",
		Summary:  "High CPU detected",
	})
	require.NoError(t, err)
}

func TestService_Report_SeverityFilter(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := reporter.NewService(srv.URL, "warning", http.DefaultClient)

	// info severity is below warning threshold
	err := svc.Report(context.Background(), &reporter.ReportData{
		ID:       "r-1",
		Severity: "info",
		Summary:  "All good",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, callCount)

	// warning severity meets threshold
	err = svc.Report(context.Background(), &reporter.ReportData{
		ID:       "r-2",
		Severity: "warning",
		Summary:  "Something happened",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestService_Report_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	svc := reporter.NewService(srv.URL, "info", http.DefaultClient)
	err := svc.Report(context.Background(), &reporter.ReportData{
		ID:       "r-1",
		Severity: "critical",
		Summary:  "Error test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook returned status 500")
}

func TestService_Report_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := reporter.NewService(srv.URL, "info", http.DefaultClient)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.Report(ctx, &reporter.ReportData{
		ID:       "r-1",
		Severity: "critical",
		Summary:  "Context test",
	})
	require.Error(t, err)
}

func TestBuildSlackPayload(t *testing.T) {
	report := &reporter.ReportData{
		ID:          "r-1",
		Trigger:     "webhook",
		Severity:    "critical",
		Summary:     "Summary text",
		Detail:      "Detailed text",
		Datasources: []string{"vm", "loki"},
		CreatedAt:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	// Test via Report to verify payload structure
	var payload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	svc := reporter.NewService(srv.URL, "info", http.DefaultClient)
	_ = svc.Report(context.Background(), report)

	assert.Contains(t, payload["text"], "critical")
	assert.Contains(t, payload["text"], "Summary text")

	blocks := payload["blocks"].([]any)
	assert.GreaterOrEqual(t, len(blocks), 3)
}
