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
	"github.com/serengeti-sh/meerkat/internal/report"
	reportMocks "github.com/serengeti-sh/meerkat/internal/report/mocks"
	reporterMocks "github.com/serengeti-sh/meerkat/internal/reporter/mocks"
)

func testRefs() inspector.DatasourceRefs {
	return func() []analyzer.DatasourceRef {
		return []analyzer.DatasourceRef{{Name: "vm", Type: "victoria-metrics"}}
	}
}

func TestService_Inspect_ReturnsPending(t *testing.T) {
	reportRepo := reportMocks.NewRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 100, 2)
	require.NoError(t, err)
	require.NoError(t, svc.Start())
	defer svc.Stop()

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

	rpt, err := svc.Inspect(context.Background(), inspector.InspectRequest{
		Query: "check for errors",
	})

	require.NoError(t, err)
	assert.Equal(t, report.StatusQueued, rpt.Status())
	assert.Equal(t, report.TriggerManual, rpt.Trigger())
	assert.NotEmpty(t, rpt.ID())

	// Wait for goroutine to finish
	time.Sleep(100 * time.Millisecond)
}

func TestService_InspectByWebhook_ReturnsPending(t *testing.T) {
	reportRepo := reportMocks.NewRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 100, 2)
	require.NoError(t, err)
	require.NoError(t, svc.Start())
	defer svc.Stop()

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

	rpt, err := svc.InspectByWebhook(context.Background(), inspector.WebhookPayload{
		Source:  "grafana",
		Alert:   "HighErrorRate",
		Message: "Error rate above 5%",
	})

	require.NoError(t, err)
	assert.Equal(t, report.StatusQueued, rpt.Status())
	assert.Equal(t, report.TriggerWebhook, rpt.Trigger())

	time.Sleep(100 * time.Millisecond)
}

func TestService_Inspect_NoDatasources(t *testing.T) {
	reportRepo := reportMocks.NewRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	emptyRefs := func() []analyzer.DatasourceRef { return nil }
	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, emptyRefs, 5*time.Minute, 100, 2)
	require.NoError(t, err)

	_, err = svc.Inspect(context.Background(), inspector.InspectRequest{Query: "test"})

	assert.Error(t, err)
}

func TestService_GetReport(t *testing.T) {
	reportRepo := reportMocks.NewRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 100, 2)
	require.NoError(t, err)

	expected := report.NewReport(report.WithID("rpt-1"), report.WithTrigger(report.TriggerManual), report.WithTriggerID("t-1"), report.WithStatus(report.StatusCompleted), report.WithSeverity(report.SeverityWarning), report.WithSummary("test"), report.WithDetail("detail"), report.WithQuery("test query"), report.WithIterations(3), report.WithCreatedAt(time.Now()))
	reportRepo.EXPECT().GetByID(mock.Anything, "rpt-1").Return(expected, nil)

	rpt, err := svc.GetReport(context.Background(), "rpt-1")

	require.NoError(t, err)
	assert.Equal(t, "rpt-1", rpt.ID())
	assert.Equal(t, report.StatusCompleted, rpt.Status())
}

func TestService_ListReports(t *testing.T) {
	reportRepo := reportMocks.NewRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 100, 2)
	require.NoError(t, err)

	r1 := report.NewReport(report.WithID("rpt-1"), report.WithTrigger(report.TriggerManual), report.WithTriggerID("t-1"), report.WithStatus(report.StatusCompleted), report.WithSeverity(report.SeverityInfo), report.WithSummary("ok"), report.WithIterations(1), report.WithCreatedAt(time.Now()))
	r2 := report.NewReport(report.WithID("rpt-2"), report.WithTrigger(report.TriggerWebhook), report.WithTriggerID("t-2"), report.WithStatus(report.StatusCompleted), report.WithSeverity(report.SeverityCritical), report.WithSummary("bad"), report.WithQuery("HighErrorRate"), report.WithIterations(5), report.WithCreatedAt(time.Now()))
	reportRepo.EXPECT().List(mock.Anything, 10).Return([]*report.Report{r1, r2}, nil)

	reports, err := svc.ListReports(context.Background(), 10)

	require.NoError(t, err)
	assert.Len(t, reports, 2)
}

func TestService_Inspect_QueueFull_Returns429(t *testing.T) {
	reportRepo := reportMocks.NewRepositoryMock(t)
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
	require.NoError(t, svc.Start())
	defer svc.Stop()

	// First request - worker picks it up and blocks
	reportRepo.EXPECT().FindActiveByQuery(mock.Anything, "manual", "query-1", mock.Anything).Return(nil, nil)
	reportRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	reportRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil) // running

	rpt1, err := svc.Inspect(context.Background(), inspector.InspectRequest{Query: "query-1"})
	require.NoError(t, err)
	assert.Equal(t, report.StatusQueued, rpt1.Status())

	// Wait for worker to pick up the job and block in Analyze
	time.Sleep(50 * time.Millisecond)

	// Second request fills the queue (channel buffer size 1)
	reportRepo.EXPECT().FindActiveByQuery(mock.Anything, "manual", "query-2", mock.Anything).Return(nil, nil)
	reportRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)

	rpt2, err := svc.Inspect(context.Background(), inspector.InspectRequest{Query: "query-2"})
	require.NoError(t, err)
	assert.Equal(t, report.StatusQueued, rpt2.Status())

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
	reportRepo := reportMocks.NewRepositoryMock(t)
	analyzerSvc := analyzerMocks.NewServiceMock(t)
	reporterSvc := reporterMocks.NewServiceMock(t)

	// Queue size 10, 2 workers
	svc, err := inspector.NewService(analyzerSvc, reportRepo, reporterSvc, testRefs(), 5*time.Minute, 10, 2)
	require.NoError(t, err)
	require.NoError(t, svc.Start())
	defer svc.Stop()

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

	reports := make([]*report.Report, 3)
	for i := range 3 {
		rpt, err := svc.Inspect(context.Background(), inspector.InspectRequest{
			Query: fmt.Sprintf("query-%d", i),
		})
		require.NoError(t, err)
		reports[i] = rpt
		assert.Equal(t, report.StatusQueued, rpt.Status())
	}

	// Wait for all workers to finish
	time.Sleep(200 * time.Millisecond)
}
