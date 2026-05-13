package inspector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/serengeti-sh/meerkat/internal/analyzer"
	apperrors "github.com/serengeti-sh/meerkat/internal/errors"
	"github.com/serengeti-sh/meerkat/internal/reporter"
)

// DatasourceRefs provides the current list of datasource references for analysis.
type DatasourceRefs func() []analyzer.DatasourceRef

type service struct {
	analyzerSvc analyzer.AnalyzerService
	reportRepo  ReportRepository
	reporterSvc reporter.ReporterService
	dsRefs      DatasourceRefs
	dedupWindow time.Duration
	queueSize   int
	workerCount int
	queue       chan *analysisJob
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
}

type analysisJob struct {
	report *Report
	input  *analyzer.AnalysisInput
}

func NewService(
	analyzerSvc analyzer.AnalyzerService,
	reportRepo ReportRepository,
	reporterSvc reporter.ReporterService,
	dsRefs DatasourceRefs,
	dedupWindow time.Duration,
	queueSize int,
	workerCount int,
) InspectorService {
	if queueSize <= 0 {
		queueSize = 1000
	}
	if workerCount <= 0 {
		workerCount = 10
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &service{
		analyzerSvc: analyzerSvc,
		reportRepo:  reportRepo,
		reporterSvc: reporterSvc,
		dsRefs:      dsRefs,
		dedupWindow: dedupWindow,
		queueSize:   queueSize,
		workerCount: workerCount,
		queue:       make(chan *analysisJob, queueSize),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start worker pool
	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	return s
}

func (s *service) worker(id int) {
	defer s.wg.Done()
	log.Printf("[inspector] worker %d started", id)

	for {
		select {
		case <-s.ctx.Done():
			log.Printf("[inspector] worker %d shutting down", id)
			return
		case job := <-s.queue:
			if job == nil {
				return
			}
			s.runAnalysis(s.ctx, job.report, job.input)
		}
	}
}

// Inspect creates a queued report and submits it to the worker pool.
func (s *service) Inspect(ctx context.Context, req InspectRequest) (*Report, apperrors.Error) {
	refs := s.dsRefs()
	if len(refs) == 0 {
		return nil, apperrors.New(apperrors.ErrInvalidInput, "no datasources configured")
	}

	query := req.Query
	if query == "" && req.MetricQuery != "" {
		query = fmt.Sprintf("Check metrics: %s", req.MetricQuery)
	}
	if req.LogQuery != "" {
		query += fmt.Sprintf("\nCheck logs: %s", req.LogQuery)
	}

	// Dedup: check for an active report with the same query
	existing, err := s.reportRepo.FindActiveByQuery(ctx, "manual", query, time.Now().Add(-s.dedupWindow))
	if err != nil {
		log.Printf("[meerkat] dedup check failed: %v", err)
	}
	if existing != nil {
		return nil, apperrors.New(apperrors.ErrConflict,
			fmt.Sprintf("a similar analysis is already in progress (report %s)", existing.ID()))
	}

	triggerID := uuid.New().String()
	report := NewReport(
		uuid.New().String(),
		"manual",
		triggerID,
		StatusQueued,
		SeverityInfo,
		"",
		"",
		query,
		nil,
		0,
		time.Now(),
	)

	if err := s.reportRepo.Create(ctx, report); err != nil {
		return nil, apperrors.New(apperrors.ErrInternal, "failed to create report")
	}

	job := &analysisJob{
		report: report,
		input: &analyzer.AnalysisInput{
			Trigger:     "manual",
			TriggerID:   triggerID,
			Query:       query,
			Datasources: refs,
		},
	}

	select {
	case s.queue <- job:
		log.Printf("[inspector] report %s queued (queue: %d/%d)", report.ID(), len(s.queue), s.queueSize)
		return report, nil
	default:
		// Queue is full — update report to failed and reject
		failedReport := NewReport(
			report.ID(), report.Trigger(), report.TriggerID(),
			StatusFailed, SeverityInfo, "Analysis queue is full, request rejected",
			"", report.Query(), report.Datasources(), 0, report.CreatedAt(),
		)
		if err := s.reportRepo.Update(ctx, failedReport); err != nil {
			log.Printf("[meerkat] failed to update report %s: %v", report.ID(), err)
		}
		return nil, apperrors.New(apperrors.ErrRateLimit, "analysis queue is full, try again later")
	}
}

// InspectByWebhook creates a queued report and submits it to the worker pool.
func (s *service) InspectByWebhook(ctx context.Context, payload WebhookPayload) (*Report, apperrors.Error) {
	refs := s.dsRefs()
	if len(refs) == 0 {
		return nil, apperrors.New(apperrors.ErrInvalidInput, "no datasources configured")
	}

	// Dedup: check for an active report with the same alert
	existing, err := s.reportRepo.FindActiveByQuery(ctx, "webhook", payload.Alert, time.Now().Add(-s.dedupWindow))
	if err != nil {
		log.Printf("[meerkat] dedup check failed: %v", err)
	}
	if existing != nil {
		return nil, apperrors.New(apperrors.ErrConflict,
			fmt.Sprintf("a similar analysis is already in progress (report %s)", existing.ID()))
	}

	triggerID := uuid.New().String()
	report := NewReport(
		uuid.New().String(),
		"webhook",
		triggerID,
		StatusQueued,
		SeverityInfo,
		"",
		"",
		payload.Alert,
		nil,
		0,
		time.Now(),
	)

	if err := s.reportRepo.Create(ctx, report); err != nil {
		return nil, apperrors.New(apperrors.ErrInternal, "failed to create report")
	}

	job := &analysisJob{
		report: report,
		input: &analyzer.AnalysisInput{
			Trigger:     "webhook",
			TriggerID:   triggerID,
			Context:     fmt.Sprintf("Source: %s\nAlert: %s\nMessage: %s\nData: %s", payload.Source, payload.Alert, payload.Message, string(payload.Data)),
			Datasources: refs,
		},
	}

	select {
	case s.queue <- job:
		log.Printf("[inspector] report %s queued (queue: %d/%d)", report.ID(), len(s.queue), s.queueSize)
		return report, nil
	default:
		failedReport := NewReport(
			report.ID(), report.Trigger(), report.TriggerID(),
			StatusFailed, SeverityInfo, "Analysis queue is full, request rejected",
			"", report.Query(), report.Datasources(), 0, report.CreatedAt(),
		)
		if err := s.reportRepo.Update(ctx, failedReport); err != nil {
			log.Printf("[meerkat] failed to update report %s: %v", report.ID(), err)
		}
		return nil, apperrors.New(apperrors.ErrRateLimit, "analysis queue is full, try again later")
	}
}

// runAnalysis executes the agent loop.
func (s *service) runAnalysis(ctx context.Context, report *Report, input *analyzer.AnalysisInput) {
	// Update status to running
	runningReport := NewReport(
		report.ID(), report.Trigger(), report.TriggerID(),
		StatusRunning, report.Severity(), report.Summary(),
		report.Detail(), report.Query(), report.Datasources(), report.Iterations(),
		report.CreatedAt(),
	)
	if err := s.reportRepo.Update(ctx, runningReport); err != nil {
		log.Printf("[meerkat] failed to update report %s to running: %v", report.ID(), err)
	}

	// Run the agent loop
	result, err := s.analyzerSvc.Analyze(ctx, input)

	var finalReport *Report
	if err != nil {
		log.Printf("[meerkat] analysis failed for report %s: %v", report.ID(), err)
		finalReport = NewReport(
			report.ID(), report.Trigger(), report.TriggerID(),
			StatusFailed, SeverityInfo, fmt.Sprintf("Analysis failed: %v", err),
			report.Detail(), report.Query(), report.Datasources(), 0, report.CreatedAt(),
		)
	} else {
		finalReport = NewReport(
			report.ID(), report.Trigger(), report.TriggerID(),
			StatusCompleted, result.Severity, result.Summary,
			result.Detail, report.Query(), result.Datasources, result.Iterations, report.CreatedAt(),
		)
	}

	// Save final result
	if err := s.reportRepo.Update(ctx, finalReport); err != nil {
		log.Printf("[meerkat] failed to update report %s: %v", report.ID(), err)
	}

	// Send to reporter channels
	if finalReport.Status() == StatusCompleted {
		if err := s.reporterSvc.Report(ctx, &reporter.ReportData{
			ID:          finalReport.ID(),
			Trigger:     finalReport.Trigger(),
			TriggerID:   finalReport.TriggerID(),
			Severity:    string(finalReport.Severity()),
			Summary:     finalReport.Summary(),
			Detail:      finalReport.Detail(),
			Datasources: finalReport.Datasources(),
			Iterations:  finalReport.Iterations(),
			CreatedAt:   finalReport.CreatedAt(),
		}); err != nil {
			log.Printf("[meerkat] failed to send report %s: %v", report.ID(), err)
		}
	}

	log.Printf("[meerkat] report %s completed: status=%s severity=%s iterations=%d",
		finalReport.ID(), finalReport.Status(), finalReport.Severity(), finalReport.Iterations())
}

func (s *service) GetReport(ctx context.Context, id string) (*Report, apperrors.Error) {
	report, err := s.reportRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.New(apperrors.ErrNotFound, "report not found")
	}
	return report, nil
}

func (s *service) ListReports(ctx context.Context, limit int) ([]*Report, apperrors.Error) {
	reports, err := s.reportRepo.List(ctx, limit)
	if err != nil {
		return nil, apperrors.New(apperrors.ErrInternal, "failed to list reports")
	}
	return reports, nil
}
