package inspector_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mandacode-labs/inspector/internal/analyzer"
	analyzerMocks "github.com/mandacode-labs/inspector/internal/analyzer/mocks"
	"github.com/mandacode-labs/inspector/internal/inspector"
	inspectorMocks "github.com/mandacode-labs/inspector/internal/inspector/mocks"
	reporterMocks "github.com/mandacode-labs/inspector/internal/reporter/mocks"
)

type stubRegistry struct {
	refs []inspector.DatasourceRef
}

func (s *stubRegistry) All() []inspector.DatasourceRef { return s.refs }

func testRegistry() inspector.DatasourceRegistry {
	return &stubRegistry{
		refs: []inspector.DatasourceRef{{Name: "vm", Type: "victoria-metrics"}},
	}
}

func TestService_Inspect_ReturnsPending(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewAnalyzerServiceMock(t)
	reporterSvc := reporterMocks.NewReporterServiceMock(t)

	svc := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRegistry())

	reportRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	// Goroutine expectations
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // running
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // completed
	analyzerSvc.EXPECT().Analyze(mock.Anything, mock.Anything).Return(&analyzer.AnalysisResult{
		Severity:   analyzer.SeverityWarning,
		Summary:    "test summary",
		Detail:     "test detail",
		Iterations: 1,
	}, nil)
	reporterSvc.EXPECT().Report(mock.Anything, mock.Anything).Return(nil)

	report, err := svc.Inspect(context.Background(), inspector.InspectRequest{
		Query: "check for errors",
	})

	require.NoError(t, err)
	assert.Equal(t, inspector.StatusPending, report.Status())
	assert.Equal(t, "manual", report.Trigger())
	assert.NotEmpty(t, report.ID())

	// Wait for goroutine to finish
	time.Sleep(100 * time.Millisecond)
}

func TestService_InspectByWebhook_ReturnsPending(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewAnalyzerServiceMock(t)
	reporterSvc := reporterMocks.NewReporterServiceMock(t)

	svc := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRegistry())

	reportRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	// Goroutine expectations
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // running
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // completed
	analyzerSvc.EXPECT().Analyze(mock.Anything, mock.Anything).Return(&analyzer.AnalysisResult{
		Severity:   analyzer.SeverityWarning,
		Summary:    "webhook analysis",
		Detail:     "test detail",
		Iterations: 1,
	}, nil)
	reporterSvc.EXPECT().Report(mock.Anything, mock.Anything).Return(nil)

	report, err := svc.InspectByWebhook(context.Background(), inspector.WebhookPayload{
		Source:  "grafana",
		Alert:   "HighErrorRate",
		Message: "Error rate above 5%",
	})

	require.NoError(t, err)
	assert.Equal(t, inspector.StatusPending, report.Status())
	assert.Equal(t, "webhook", report.Trigger())

	time.Sleep(100 * time.Millisecond)
}

func TestService_Inspect_NoDatasources(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewAnalyzerServiceMock(t)
	reporterSvc := reporterMocks.NewReporterServiceMock(t)

	svc := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, &stubRegistry{})

	_, err := svc.Inspect(context.Background(), inspector.InspectRequest{Query: "test"})

	assert.Error(t, err)
}

func TestService_GetReport(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewAnalyzerServiceMock(t)
	reporterSvc := reporterMocks.NewReporterServiceMock(t)

	svc := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRegistry())

	expected := inspector.NewReport("rpt-1", "manual", "t-1", inspector.StatusCompleted, inspector.SeverityWarning, "test", "detail", nil, 3, time.Now())
	reportRepo.EXPECT().GetByID(mock.Anything, "rpt-1").Return(expected, nil)

	report, err := svc.GetReport(context.Background(), "rpt-1")

	require.NoError(t, err)
	assert.Equal(t, "rpt-1", report.ID())
	assert.Equal(t, inspector.StatusCompleted, report.Status())
}

func TestService_ListReports(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewAnalyzerServiceMock(t)
	reporterSvc := reporterMocks.NewReporterServiceMock(t)

	svc := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRegistry())

	r1 := inspector.NewReport("rpt-1", "manual", "t-1", inspector.StatusCompleted, inspector.SeverityInfo, "ok", "", nil, 1, time.Now())
	r2 := inspector.NewReport("rpt-2", "webhook", "t-2", inspector.StatusCompleted, inspector.SeverityCritical, "bad", "", nil, 5, time.Now())
	reportRepo.EXPECT().List(mock.Anything, 10).Return([]*inspector.Report{r1, r2}, nil)

	reports, err := svc.ListReports(context.Background(), 10)

	require.NoError(t, err)
	assert.Len(t, reports, 2)
}
