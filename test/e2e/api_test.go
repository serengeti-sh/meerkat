package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_HealthCheck(t *testing.T) {
	suite := SetupSuite(t)
	suite.BaseURL = "http://localhost:8080"

	resp, err := suite.Get("/v1/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, suite.ReadJSON(resp, &result))
	assert.Equal(t, "ok", result["status"])
}

func TestE2E_Datasource_CRUD(t *testing.T) {
	suite := SetupSuite(t)
	suite.BaseURL = "http://localhost:8080"

	t.Run("list empty datasources", func(t *testing.T) {
		resp, err := suite.Get("/v1/datasources")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("create datasource", func(t *testing.T) {
		body := map[string]string{
			"name": "test-vm",
			"type": "victoria-metrics",
			"url":  "http://localhost:8428",
		}
		resp, err := suite.Post("/v1/datasources", body)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var result map[string]any
		require.NoError(t, suite.ReadJSON(resp, &result))
		assert.NotEmpty(t, result["id"])
		assert.Equal(t, "test-vm", result["name"])
		assert.Equal(t, "victoria-metrics", result["type"])
	})

	t.Run("list datasources after create", func(t *testing.T) {
		resp, err := suite.Get("/v1/datasources")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]any
		require.NoError(t, suite.ReadJSON(resp, &result))
		assert.GreaterOrEqual(t, len(result), 1)
	})
}

func TestE2E_Inspect_Accepted(t *testing.T) {
	suite := SetupSuite(t)
	suite.BaseURL = "http://localhost:8080"

	// Create a datasource first
	_, err := suite.Post("/v1/datasources", map[string]string{
		"name": "inspect-test-vm",
		"type": "victoria-metrics",
		"url":  "http://localhost:8428",
	})
	require.NoError(t, err)

	t.Run("inspect returns 202 accepted", func(t *testing.T) {
		body := map[string]string{
			"query": "Check for error spikes in the last hour",
		}
		resp, err := suite.Post("/v1/inspect", body)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusAccepted, resp.StatusCode)

		var result map[string]any
		require.NoError(t, suite.ReadJSON(resp, &result))
		assert.NotEmpty(t, result["id"])
		assert.Equal(t, "pending", result["status"])
		assert.Equal(t, "manual", result["trigger"])
	})
}

func TestE2E_Reports_List(t *testing.T) {
	suite := SetupSuite(t)
	suite.BaseURL = "http://localhost:8080"

	resp, err := suite.Get("/v1/reports?limit=10")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, suite.ReadJSON(resp, &result))
	// May be empty or have reports from previous tests
	assert.NotNil(t, result)
}
