package inspector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/serengeti-sh/meerkat/internal/analyzer"
	apperrors "github.com/serengeti-sh/meerkat/internal/apperrors"
	"github.com/serengeti-sh/meerkat/internal/reporter"
)

const (
	defaultQueueSize   = 1000
	defaultWorkerCount = 10
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
	cancel      context.CancelFunc
}

var _ InspectorService = (*service)(nil)

var _ InspectorService = (*service)(nil)

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
	if analyzerSvc == nil {
		panic("inspector: analyzerSvc is required")
	}
	if reportRepo == nil {
		panic("inspector: reportRepo is required")
	}
	if dsRefs == nil {
		panic("inspector: dsRefs is required")
	}
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}
	if workerCount <= 0 {
		workerCount = defaultWorkerCount
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
		cancel:      cancel,
	}

	// Start worker pool — pass ctx explicitly as function parameter
	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i, ctx)
	}

	return s
}

// Inspect creates a queued report and submits it to the worker pool.
func (s *service) Inspect(ctx context.Context, req InspectRequest) (*Report, apperrors.Error) {
	query := req.Query
	if query == "" && req.MetricQuery != "" {
		query = fmt.Sprintf("Check metrics: %s", req.MetricQuery)
	}
	if req.LogQuery != "" {
		query += fmt.Sprintf("\nCheck logs: %s", req.LogQuery)
	}

	return s.enqueue(ctx, TriggerManual, query, "")
}

// InspectByWebhook creates a queued report and submits it to the worker pool.
func (s *service) InspectByWebhook(ctx context.Context, payload WebhookPayload) (*Report, apperrors.Error) {
	contextStr := fmt.Sprintf("Source: %s\nAlert: %s\nMessage: %s\nData: %s",
		payload.Source, payload.Alert, payload.Message, string(payload.Data))

	return s.enqueue(ctx, TriggerWebhook, payload.Alert, contextStr)
}

// enqueue is the shared logic for Inspect and InspectByWebhook.
func (s *service) enqueue(ctx context.Context, trigger TriggerType, query, contextStr string) (*Report, apperrors.Error) {
	refs := s.dsRefs()
	if len(refs) == 0 {
		return nil, apperrors.New(apperrors.ErrInvalidInput, "no datasources configured")
	}

	// Dedup: check for an active report with the same query
	existing, err := s.reportRepo.FindActiveByQuery(ctx, string(trigger), query, time.Now().Add(-s.dedupWindow))
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
		trigger,
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
		return nil, apperrors.Wrap(apperrors.ErrInternal, "failed to create report", err)
	}

	input := &analyzer.AnalysisInput{
		Trigger:     string(trigger),
		TriggerID:   triggerID,
		Query:       query,
		Context:     contextStr,
		Datasources: refs,
	}
	if contextStr == "" {
		input.Context = "" // ensure zero value when not set
	}

	job := &analysisJob{
		report: report,
		input:  input,
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

func (s *service) GetReport(ctx context.Context, id string) (*Report, apperrors.Error) {
	report, err := s.reportRepo.GetByID(ctx, id)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrNotFound, "report not found", err)
	}
	return report, nil
}

func (s *service) ListReports(ctx context.Context, limit int) ([]*Report, apperrors.Error) {
	reports, err := s.reportRepo.List(ctx, limit)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternal, "failed to list reports", err)
	}
	return reports, nil
}

func (s *service) Stop() {
	s.cancel()
	s.wg.Wait()
	log.Printf("[inspector] all workers stopped")
}
