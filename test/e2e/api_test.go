package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_HealthCheck(t *testing.T) {
	suite := SetupSuite(t)

	resp, err := suite.Get("/v1/health")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, suite.ReadJSON(resp, &result))
	assert.Equal(t, "ok", result["status"])
}

func TestE2E_Datasources(t *testing.T) {
	suite := SetupSuite(t)

	t.Run("list datasources has test-vm", func(t *testing.T) {
		resp, err := suite.Get("/v1/datasources")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result []map[string]any
		require.NoError(t, suite.ReadJSON(resp, &result))
		assert.GreaterOrEqual(t, len(result), 1)

		// Find test-vm
		found := false
		for _, ds := range result {
			if ds["name"] == "test-vm" {
				found = true
				break
			}
		}
		assert.True(t, found, "test-vm datasource should be registered")
	})
}

func TestE2E_Reports_List_Empty(t *testing.T) {
	suite := SetupSuite(t)

	resp, err := suite.Get("/v1/reports?limit=10")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result []map[string]any
	require.NoError(t, suite.ReadJSON(resp, &result))
	assert.NotNil(t, result)
}
