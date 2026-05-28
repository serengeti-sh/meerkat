package httphandler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/httphandler"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	inspectorMocks "github.com/serengeti-sh/meerkat/internal/inspector/mocks"
	"github.com/serengeti-sh/meerkat/internal/report"
)

func TestHandler_Health(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, "ok", result["status"])
}

func TestHandler_Inspect(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("Inspect", mock.Anything, inspector.InspectRequest{
		MetricQuery: "up",
		LogQuery:    "",
		Query:       "check status",
	}).Return(
		report.NewReport(report.WithID("r-1"), report.WithTrigger(report.TriggerManual), report.WithStatus(report.StatusPending), report.WithSeverity(report.SeverityInfo), report.WithSummary("all ok"), report.WithQuery("check status"), report.WithDatasources([]string{"vm"}), report.WithIterations(1), report.WithCreatedAt(time.Now())),
		nil,
	)

	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"metric_query": "up", "query": "check status"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/inspect", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, "r-1", result["id"])
	assert.Equal(t, "pending", result["status"])
}

func TestHandler_Inspect_InvalidBody(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/inspect", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_Webhook(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("InspectByWebhook", mock.Anything, inspector.WebhookPayload{
		Source:  "grafana",
		Alert:   "High CPU",
		Message: "CPU > 80%",
		Data:    json.RawMessage(`{"value": 85}`),
	}).Return(
		report.NewReport(report.WithID("r-2"), report.WithTrigger(report.TriggerWebhook), report.WithStatus(report.StatusCompleted), report.WithSeverity(report.SeverityWarning), report.WithSummary("high cpu"), report.WithDatasources([]string{"vm"}), report.WithIterations(1), report.WithCreatedAt(time.Now())),
		nil,
	)

	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"source": "grafana", "alert": "High CPU", "message": "CPU > 80%", "data": {"value": 85}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/webhook", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, "r-2", result["id"])
	assert.Equal(t, "warning", result["severity"])
}

func TestHandler_Webhook_InvalidBody(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/v1/webhook", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_ListReports(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("ListReports", mock.Anything, 50).Return(
		[]*report.Report{
			report.NewReport(report.WithID("r-1"), report.WithTrigger(report.TriggerManual), report.WithStatus(report.StatusCompleted), report.WithSeverity(report.SeverityInfo), report.WithSummary("ok"), report.WithDatasources([]string{}), report.WithIterations(1), report.WithCreatedAt(time.Now())),
		},
		nil,
	)

	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/reports", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result, 1)
	assert.Equal(t, "r-1", result[0]["id"])
}

func TestHandler_ListReports_WithLimit(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("ListReports", mock.Anything, 10).Return(
		[]*report.Report{},
		nil,
	)

	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/reports?limit=10", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestHandler_GetReport(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("GetReport", mock.Anything, "r-1").Return(
		report.NewReport(report.WithID("r-1"), report.WithTrigger(report.TriggerManual), report.WithStatus(report.StatusCompleted), report.WithSeverity(report.SeverityCritical), report.WithSummary("critical issue"), report.WithDetail("details"), report.WithDatasources([]string{"vm"}), report.WithIterations(3), report.WithCreatedAt(time.Now())),
		nil,
	)

	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/r-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var result map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	assert.Equal(t, "r-1", result["id"])
	assert.Equal(t, "critical", result["severity"])
	assert.Equal(t, float64(3), result["iterations"])
}
