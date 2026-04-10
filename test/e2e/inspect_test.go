package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/inspector/test/e2e/mock"
)

func TestE2E_Inspect_Manual(t *testing.T) {
	suite := SetupSuite(t)
	// Reset mock state
	suite.MockOpenAI.Reset()

	t.Run("inspect triggers analysis and returns completed report", func(t *testing.T) {
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
		assert.Equal(t, "pending", createResult["status"])
		assert.Equal(t, "manual", createResult["trigger"])

		reportID := createResult["id"].(string)

		// Wait for analysis to complete
		report, err := suite.WaitForReportStatus(reportID, "completed", 15*time.Second)
		require.NoError(t, err)

		assert.Equal(t, "completed", report["status"])
		assert.Equal(t, "critical", report["severity"])
		assert.NotEmpty(t, report["summary"])
		assert.Contains(t, report["summary"], "Error spike")

		// Verify mock OpenAI was called (2 iterations: tool call + final)
		assert.GreaterOrEqual(t, len(suite.MockOpenAI.Calls), 2)

		// Verify mock Prometheus received a query
		assert.GreaterOrEqual(t, len(suite.MockPrometheus.Queries), 1)
	})
}

func TestE2E_Inspect_Webhook(t *testing.T) {
	suite := SetupSuite(t)
	suite.MockOpenAI.Reset()

	t.Run("webhook triggers analysis and returns completed report", func(t *testing.T) {
		payload := map[string]any{
			"alert":   "HighErrorRate",
			"message": "Error rate exceeded 5% threshold",
			"source":  "grafana",
			"data": map[string]any{
				"threshold": "5%",
				"current":   "12.3%",
			},
		}
		resp, err := suite.Post("/v1/webhook/grafana", payload)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var createResult map[string]any
		require.NoError(t, suite.ReadJSON(resp, &createResult))
		assert.NotEmpty(t, createResult["id"])
		assert.Equal(t, "pending", createResult["status"])
		assert.Equal(t, "webhook", createResult["trigger"])

		reportID := createResult["id"].(string)

		// Wait for analysis to complete
		report, err := suite.WaitForReportStatus(reportID, "completed", 15*time.Second)
		require.NoError(t, err)

		assert.Equal(t, "completed", report["status"])
		assert.NotEmpty(t, report["summary"])
	})
}

func TestE2E_Inspect_DirectAnswer(t *testing.T) {
	suite := SetupSuite(t)
	// Configure mock to answer immediately without tool calls
	suite.MockOpenAI.Reset()
	suite.MockOpenAI.SetResponses(
		mock.DirectAnswerScenario("info", "All systems normal", "No anomalies detected in the monitored services."),
	)

	t.Run("inspect with direct answer (no tool calls)", func(t *testing.T) {
		body := map[string]string{
			"query": "Check system health",
		}
		resp, err := suite.Post("/v1/inspect", body)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var createResult map[string]any
		require.NoError(t, suite.ReadJSON(resp, &createResult))
		reportID := createResult["id"].(string)

		report, err := suite.WaitForReportStatus(reportID, "completed", 10*time.Second)
		require.NoError(t, err)

		assert.Equal(t, "completed", report["status"])
		assert.Equal(t, "info", report["severity"])
		assert.Contains(t, report["summary"], "All systems normal")

		// Should only be 1 call since no tools were used
		assert.Equal(t, 1, len(suite.MockOpenAI.Calls))
	})
}
