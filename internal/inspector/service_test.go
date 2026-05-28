package inspector_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	analyzerMocks "github.com/serengeti-sh/meerkat/internal/analyzer/mocks"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	inspectorMocks "github.com/serengeti-sh/meerkat/internal/inspector/mocks"
	reporterMocks "github.com/serengeti-sh/meerkat/internal/reporter/mocks"
)

func testRefs() inspector.DatasourceRefs {
	return func() []analyzer.DatasourceRef {
		return []analyzer.DatasourceRef{{Name: "vm", Type: "victoria-metrics"}}
	}
}

func TestService_Inspect_ReturnsPending(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 100, 2)
	require.NoError(t, err)

	reportRepo.EXPECT().FindActiveByQuery(mock.Anything, "manual", "check for errors", mock.Anything).Return(nil, nil)
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
	assert.Equal(t, inspector.StatusQueued, report.Status())
	assert.Equal(t, inspector.TriggerManual, report.Trigger())
	assert.NotEmpty(t, report.ID())

	// Wait for goroutine to finish
	time.Sleep(100 * time.Millisecond)
}

func TestService_InspectByWebhook_ReturnsPending(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 100, 2)
	require.NoError(t, err)

	reportRepo.EXPECT().FindActiveByQuery(mock.Anything, "webhook", "HighErrorRate", mock.Anything).Return(nil, nil)
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
	assert.Equal(t, inspector.StatusQueued, report.Status())
	assert.Equal(t, inspector.TriggerWebhook, report.Trigger())

	time.Sleep(100 * time.Millisecond)
}

func TestService_Inspect_NoDatasources(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	emptyRefs := func() []analyzer.DatasourceRef { return nil }
	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, emptyRefs, 5*time.Minute, 100, 2)
	require.NoError(t, err)

	_, err = svc.Inspect(context.Background(), inspector.InspectRequest{Query: "test"})

	assert.Error(t, err)
}

func TestService_GetReport(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 100, 2)
	require.NoError(t, err)

	expected := inspector.NewReport("rpt-1", inspector.TriggerManual, "t-1", inspector.StatusCompleted, inspector.SeverityWarning, "test", "detail", "test query", nil, 3, time.Now())
	reportRepo.EXPECT().GetByID(mock.Anything, "rpt-1").Return(expected, nil)

	report, err := svc.GetReport(context.Background(), "rpt-1")

	require.NoError(t, err)
	assert.Equal(t, "rpt-1", report.ID())
	assert.Equal(t, inspector.StatusCompleted, report.Status())
}

func TestService_ListReports(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 100, 2)
	require.NoError(t, err)

	r1 := inspector.NewReport("rpt-1", inspector.TriggerManual, "t-1", inspector.StatusCompleted, inspector.SeverityInfo, "ok", "", "", nil, 1, time.Now())
	r2 := inspector.NewReport("rpt-2", inspector.TriggerWebhook, "t-2", inspector.StatusCompleted, inspector.SeverityCritical, "bad", "", "HighErrorRate", nil, 5, time.Now())
	reportRepo.EXPECT().List(mock.Anything, 10).Return([]*inspector.Report{r1, r2}, nil)

	reports, err := svc.ListReports(context.Background(), 10)

	require.NoError(t, err)
	assert.Len(t, reports, 2)
}

func TestService_Inspect_QueueFull_Returns429(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	// Queue size 1, 1 worker that blocks
	blockCh := make(chan struct{})
	callCount := 0
	analyzerSvc.EXPECT().Analyze(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, input *analyzer.AnalysisInput) (*analyzer.AnalysisResult, error) {
		callCount++
		if callCount == 1 {
			<-blockCh // First call blocks until test signals
		}
		return &analyzer.AnalysisResult{Severity: analyzer.SeverityInfo, Summary: "done", Iterations: 1}, nil
	}).Twice()

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 1, 1)
	require.NoError(t, err)

	// First request - worker picks it up and blocks
	reportRepo.EXPECT().FindActiveByQuery(mock.Anything, "manual", "query-1", mock.Anything).Return(nil, nil)
	reportRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // running

	report1, err := svc.Inspect(context.Background(), inspector.InspectRequest{Query: "query-1"})
	require.NoError(t, err)
	assert.Equal(t, inspector.StatusQueued, report1.Status())

	// Wait for worker to pick up the job and block in Analyze
	time.Sleep(50 * time.Millisecond)

	// Second request fills the queue (channel buffer size 1)
	reportRepo.EXPECT().FindActiveByQuery(mock.Anything, "manual", "query-2", mock.Anything).Return(nil, nil)
	reportRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	report2, err := svc.Inspect(context.Background(), inspector.InspectRequest{Query: "query-2"})
	require.NoError(t, err)
	assert.Equal(t, inspector.StatusQueued, report2.Status())

	// Third request should be rejected (queue full)
	reportRepo.EXPECT().FindActiveByQuery(mock.Anything, "manual", "query-3", mock.Anything).Return(nil, nil)
	reportRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // failed status update

	_, err = svc.Inspect(context.Background(), inspector.InspectRequest{Query: "query-3"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "queue is full")

	// Expectations for when worker completes jobs after unblocking
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // completed for job 1
	reporterSvc.EXPECT().Report(mock.Anything, mock.Anything).Return(nil)
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // running for job 2
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // completed for job 2
	reporterSvc.EXPECT().Report(mock.Anything, mock.Anything).Return(nil)

	// Unblock the worker
	close(blockCh)
	time.Sleep(200 * time.Millisecond)
}

func TestService_WorkerPool_ProcessesMultipleJobs(t *testing.T) {
	reportRepo := inspectorMocks.NewReportRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	// Queue size 10, 2 workers
	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 10, 2)
	require.NoError(t, err)

	// Submit 3 jobs
	for range 3 {
		reportRepo.EXPECT().FindActiveByQuery(mock.Anything, "manual", mock.Anything, mock.Anything).Return(nil, nil)
		reportRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	}

	// Each job gets processed: running + completed updates
	for i := range 3 {
		reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // running
		reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // completed
		analyzerSvc.EXPECT().Analyze(mock.Anything, mock.Anything).Return(&analyzer.AnalysisResult{
			Severity:   analyzer.SeverityInfo,
			Summary:    fmt.Sprintf("result-%d", i),
			Iterations: 1,
		}, nil)
		reporterSvc.EXPECT().Report(mock.Anything, mock.Anything).Return(nil)
	}

	reports := make([]*inspector.Report, 3)
	for i := range 3 {
		report, err := svc.Inspect(context.Background(), inspector.InspectRequest{
			Query: fmt.Sprintf("query-%d", i),
		})
		require.NoError(t, err)
		reports[i] = report
		assert.Equal(t, inspector.StatusQueued, report.Status())
	}

	// Wait for all workers to finish
	time.Sleep(200 * time.Millisecond)
}
