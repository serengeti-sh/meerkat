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

	"github.com/serengeti-sh/meerkat/internal/errs"
	"github.com/serengeti-sh/meerkat/internal/httphandler"
	"github.com/serengeti-sh/meerkat/internal/inspect"
	inspectorMocks "github.com/serengeti-sh/meerkat/internal/inspect/mocks"
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
	mockSvc.On("Inspect", mock.Anything, inspect.Request{
		MetricQuery: "up",
		LogQuery:    "",
		Query:       "check status",
	}).Return(
		&report.Report{ID: "r-1", Trigger: report.TriggerManual, Status: report.StatusPending, Severity: report.SeverityInfo, Summary: "all ok", Query: "check status", Datasources: []string{"vm"}, Iterations: 1, CreatedAt: time.Now()},
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

func TestHandler_Inspect_ServiceError(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("Inspect", mock.Anything, inspect.Request{
		Query: "check status",
	}).Return(
		nil,
		errs.New(errs.ErrInternal, "database error"),
	)

	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	body := `{"query": "check status"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/inspect", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_Webhook(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("InspectByWebhook", mock.Anything, inspect.WebhookPayload{
		Source:  "grafana",
		Alert:   "High CPU",
		Message: "CPU > 80%",
		Data:    json.RawMessage(`{"value": 85}`),
	}).Return(
		&report.Report{ID: "r-2", Trigger: report.TriggerWebhook, Status: report.StatusCompleted, Severity: report.SeverityWarning, Summary: "high cpu", Datasources: []string{"vm"}, Iterations: 1, CreatedAt: time.Now()},
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
			&report.Report{ID: "r-1", Trigger: report.TriggerManual, Status: report.StatusCompleted, Severity: report.SeverityInfo, Summary: "ok", Datasources: []string{}, Iterations: 1, CreatedAt: time.Now()},
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

func TestHandler_ListReports_ServiceError(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("ListReports", mock.Anything, 50).Return(
		nil,
		errs.New(errs.ErrInternal, "database error"),
	)

	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/reports", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandler_GetReport(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("GetReport", mock.Anything, "r-1").Return(
		&report.Report{ID: "r-1", Trigger: report.TriggerManual, Status: report.StatusCompleted, Severity: report.SeverityCritical, Summary: "critical issue", Detail: "details", Datasources: []string{"vm"}, Iterations: 3, CreatedAt: time.Now()},
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

func TestHandler_GetReport_NotFound(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("GetReport", mock.Anything, "r-missing").Return(
		nil,
		errs.New(errs.ErrNotFound, "report not found"),
	)

	h, err := httphandler.New(mockSvc)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/reports/r-missing", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_New_NilInspector(t *testing.T) {
	_, err := httphandler.New(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inspectorSvc is required")
}
