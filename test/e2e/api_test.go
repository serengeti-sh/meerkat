package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_Health(t *testing.T) {
	suite := SetupSuite(t)

	resp, err := suite.Get("/v1/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	require.NoError(t, suite.ReadJSON(resp, &result))
	assert.Equal(t, "ok", result["status"])
}

func TestE2E_Inspect_Manual(t *testing.T) {
	suite := SetupSuite(t)

	body := map[string]string{
		"query": "Check for error spikes in the last hour",
	}
	resp, err := suite.Post("/v1/inspect", body)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var createResult map[string]any
	require.NoError(t, suite.ReadJSON(resp, &createResult))
	assert.NotEmpty(t, createResult["id"])
	assert.Equal(t, "queued", createResult["status"])
	assert.Equal(t, "manual", createResult["trigger"])

	reportID := createResult["id"].(string)

	// Wait for analysis to complete
	report, err := suite.WaitForReportStatus(reportID, "completed", 15*time.Second)
	require.NoError(t, err)

	assert.Equal(t, "completed", report["status"])
	assert.Equal(t, "critical", report["severity"])
	assert.NotEmpty(t, report["summary"])
}

func TestE2E_InspectByWebhook(t *testing.T) {
	suite := SetupSuite(t)

	payload := map[string]any{
		"alert":   "HighErrorRate",
		"message": "Error rate exceeded 5% threshold",
		"source":  "grafana",
		"data": map[string]any{
			"threshold": "5%",
			"current":   "12.3%",
		},
	}
	resp, err := suite.Post("/v1/webhook", payload)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var createResult map[string]any
	require.NoError(t, suite.ReadJSON(resp, &createResult))
	assert.NotEmpty(t, createResult["id"])
	assert.Equal(t, "queued", createResult["status"])
	assert.Equal(t, "webhook", createResult["trigger"])
}

func TestE2E_ListReports(t *testing.T) {
	suite := SetupSuite(t)

	// Create a report first
	resp, err := suite.Post("/v1/inspect", map[string]string{
		"query": "List reports test",
	})
	require.NoError(t, err)
	_ = resp.Body.Close()

	// List reports
	resp, err = suite.Get("/v1/reports?limit=10")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, suite.ReadJSON(resp, &result))
	assert.NotNil(t, result)
}
