package httphandler_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/errs"
	"github.com/serengeti-sh/meerkat/internal/httphandler"
	"github.com/serengeti-sh/meerkat/internal/inspect"
	inspectorMocks "github.com/serengeti-sh/meerkat/internal/inspect/mocks"
	"github.com/serengeti-sh/meerkat/internal/report"
	"github.com/serengeti-sh/meerkat/pkg/api"
)

func newTestLogger() zerolog.Logger {
	return zerolog.New(nil)
}

func TestHandler_GetHealth(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	h := httphandler.New(mockSvc, newTestLogger())

	res, err := h.GetHealth(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "ok", res.Status)
}

func TestHandler_CreateInspect(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("Inspect", mock.Anything, inspect.Request{
		MetricQuery: "up",
		LogQuery:    "",
		Query:       "check status",
	}).Return(
		&report.Report{ID: "r-1", Trigger: report.TriggerManual, Status: report.StatusPending, Severity: report.SeverityInfo, Summary: "all ok", Query: "check status", Datasources: []string{"vm"}, Iterations: 1, CreatedAt: time.Now()},
		nil,
	)

	h := httphandler.New(mockSvc, newTestLogger())
	res, err := h.CreateInspect(context.Background(), &api.CreateInspectReq{
		Query:       api.NewOptString("check status"),
		MetricQuery: api.NewOptString("up"),
	})

	require.NoError(t, err)
	ok, isOK := res.(*api.ReportResponse)
	require.True(t, isOK)
	assert.Equal(t, "r-1", ok.ID)
	assert.Equal(t, api.ReportResponseStatus(report.StatusPending), ok.Status)
}

func TestHandler_CreateInspect_ServiceError(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("Inspect", mock.Anything, inspect.Request{
		Query: "check status",
	}).Return(
		nil,
		errs.New(errs.ErrInternal, "database error"),
	)

	h := httphandler.New(mockSvc, newTestLogger())
	res, err := h.CreateInspect(context.Background(), &api.CreateInspectReq{
		Query: api.NewOptString("check status"),
	})

	require.NoError(t, err)
	errRes, isErr := res.(*api.ErrorStatusCode)
	require.True(t, isErr)
	assert.Equal(t, 500, errRes.StatusCode)
}

func TestHandler_ReceiveWebhook(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("InspectByWebhook", mock.Anything, inspect.WebhookPayload{
		Source:  "grafana",
		Alert:   "High CPU",
		Message: "CPU > 80%",
	}).Return(
		&report.Report{ID: "r-2", Trigger: report.TriggerWebhook, Status: report.StatusCompleted, Severity: report.SeverityWarning, Summary: "high cpu", Datasources: []string{"vm"}, Iterations: 1, CreatedAt: time.Now()},
		nil,
	)

	h := httphandler.New(mockSvc, newTestLogger())
	res, err := h.ReceiveWebhook(context.Background(), &api.ReceiveWebhookReq{
		Source:  api.NewOptString("grafana"),
		Alert:   api.NewOptString("High CPU"),
		Message: api.NewOptString("CPU > 80%"),
	})

	require.NoError(t, err)
	ok, isOK := res.(*api.ReportResponse)
	require.True(t, isOK)
	assert.Equal(t, "r-2", ok.ID)
}

func TestHandler_ListReports(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("ListReports", mock.Anything, 50).Return(
		[]*report.Report{
			{ID: "r-1", Trigger: report.TriggerManual, Status: report.StatusCompleted, Severity: report.SeverityInfo, Summary: "ok", Datasources: []string{}, Iterations: 1, CreatedAt: time.Now()},
		},
		nil,
	)

	h := httphandler.New(mockSvc, newTestLogger())
	res, err := h.ListReports(context.Background(), api.ListReportsParams{})

	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "r-1", res[0].ID)
}

func TestHandler_ListReports_WithLimit(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("ListReports", mock.Anything, 10).Return(
		[]*report.Report{},
		nil,
	)

	h := httphandler.New(mockSvc, newTestLogger())
	res, err := h.ListReports(context.Background(), api.ListReportsParams{
		Limit: api.NewOptInt(10),
	})

	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestHandler_ListReports_ServiceError(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("ListReports", mock.Anything, 50).Return(
		nil,
		errs.New(errs.ErrInternal, "database error"),
	)

	h := httphandler.New(mockSvc, newTestLogger())
	_, err := h.ListReports(context.Background(), api.ListReportsParams{})

	require.Error(t, err)
}

func TestHandler_GetReport(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("GetReport", mock.Anything, "r-1").Return(
		&report.Report{ID: "r-1", Trigger: report.TriggerManual, Status: report.StatusCompleted, Severity: report.SeverityCritical, Summary: "critical issue", Detail: "details", Datasources: []string{"vm"}, Iterations: 3, CreatedAt: time.Now()},
		nil,
	)

	h := httphandler.New(mockSvc, newTestLogger())
	res, err := h.GetReport(context.Background(), api.GetReportParams{ID: "r-1"})

	require.NoError(t, err)
	ok, isOK := res.(*api.ReportResponse)
	require.True(t, isOK)
	assert.Equal(t, "r-1", ok.ID)
	assert.Equal(t, api.ReportResponseSeverity(report.SeverityCritical), ok.Severity.Value)
	assert.Equal(t, 3, ok.Iterations.Value)
}

func TestHandler_GetReport_NotFound(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("GetReport", mock.Anything, "r-missing").Return(
		nil,
		errs.New(errs.ErrNotFound, "report not found"),
	)

	h := httphandler.New(mockSvc, newTestLogger())
	res, err := h.GetReport(context.Background(), api.GetReportParams{ID: "r-missing"})

	require.NoError(t, err)
	errRes, isErr := res.(*api.ErrorStatusCode)
	require.True(t, isErr)
	assert.Equal(t, 404, errRes.StatusCode)
}

func TestHandler_New_NilInspector(t *testing.T) {
	assert.Panics(t, func() {
		httphandler.New(nil, newTestLogger())
	})
}
