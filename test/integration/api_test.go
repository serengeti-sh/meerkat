package integration

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
