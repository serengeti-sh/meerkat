package inspect

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/ent"
	errs "github.com/serengeti-sh/meerkat/internal/errs"
	"github.com/serengeti-sh/meerkat/internal/vectorsclient"
	"github.com/serengeti-sh/meerkat/internal/notify"
	"github.com/serengeti-sh/meerkat/internal/report"
)

// Service orchestrates inspections.
type Service interface {
	Start() error
	Stop()
	Inspect(ctx context.Context, req Request) (*report.Report, error)
	InspectByWebhook(ctx context.Context, payload WebhookPayload) (*report.Report, error)
	GetReport(ctx context.Context, id string) (*report.Report, error)
	ListReports(ctx context.Context, limit int) ([]*report.Report, error)
}

const (
	defaultQueueSize   = 1000
	defaultWorkerCount = 10
	logsContextWindow  = 15 * time.Minute
	logsContextLimit   = 20
)

// ServiceOption configures the inspector service.
type ServiceOption func(*service)

// WithLogsClient sets the vectors client for online log retrieval.
func WithLogsClient(client vectorsclient.Client) ServiceOption {
	return func(s *service) {
		s.logsClient = client
	}
}

// DatasourceRefs provides the current list of datasource references for analysis.
type DatasourceRefs func() []analyzer.DatasourceRef

type service struct {
	analyzerSvc analyzer.Service
	reportRepo  report.Repository
	reporterSvc notify.Service
	logsClient  vectorsclient.Client
	dsRefs      DatasourceRefs
	dedupWindow time.Duration
	queueSize   int
	workerCount int
	queue       chan *analysisJob
	wg          sync.WaitGroup
	cancel      context.CancelFunc
	startOnce   sync.Once
}

var _ Service = (*service)(nil)

type analysisJob struct {
	report *report.Report
	input  *analyzer.AnalysisInput
}

func NewService(
	analyzerSvc analyzer.Service,
	reportRepo report.Repository,
	reporterSvc notify.Service,
	dsRefs DatasourceRefs,
	dedupWindow time.Duration,
	queueSize int,
	workerCount int,
	opts ...ServiceOption,
) (*service, error) {
	if analyzerSvc == nil {
		return nil, fmt.Errorf("analyzer service is required")
	}
	if reportRepo == nil {
		return nil, fmt.Errorf("report repository is required")
	}
	if dsRefs == nil {
		return nil, fmt.Errorf("datasource refs are required")
	}
	if reporterSvc == nil {
		return nil, fmt.Errorf("reporter service is required")
	}

	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}
	if workerCount <= 0 {
		workerCount = defaultWorkerCount
	}

	s := &service{
		analyzerSvc: analyzerSvc,
		reportRepo:  reportRepo,
		reporterSvc: reporterSvc,
		dsRefs:      dsRefs,
		dedupWindow: dedupWindow,
		queueSize:   queueSize,
		workerCount: workerCount,
		queue:       make(chan *analysisJob, queueSize),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// Start launches the worker pool. It is safe to call only once.
func (s *service) Start() error {
	var startErr error
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel

		for i := 0; i < s.workerCount; i++ {
			s.wg.Add(1)
			go s.worker(i, ctx)
		}
	})
	return startErr
}

// Inspect creates a queued report and submits it to the worker pool.
func (s *service) Inspect(ctx context.Context, req Request) (*report.Report, error) {
	query := req.Query
	if query == "" && req.MetricQuery != "" {
		query = fmt.Sprintf("Check metrics: %s", req.MetricQuery)
	}
	if req.LogQuery != "" {
		query += fmt.Sprintf("\nCheck logs: %s", req.LogQuery)
	}

	return s.enqueue(ctx, report.TriggerManual, query, "")
}

// InspectByWebhook creates a queued report and submits it to the worker pool.
func (s *service) InspectByWebhook(ctx context.Context, payload WebhookPayload) (*report.Report, error) {
	contextStr := fmt.Sprintf("Source: %s\nAlert: %s\nMessage: %s\nData: %s",
		payload.Source, payload.Alert, payload.Message, string(payload.Data))

	// Online Retrieval: fetch recent log context from vectors
	if s.logsClient != nil {
		logsCtx := s.fetchLogsContext(ctx, payload)
		if logsCtx != "" {
			contextStr += "\n\n=== Recent Log Context ===\n" + logsCtx
		}
	}

	return s.enqueue(ctx, report.TriggerWebhook, payload.Alert, contextStr)
}

// fetchLogsContext extracts a service name from the webhook payload and queries
// the vectors index for recent log entries. Returns empty string on error.
func (s *service) fetchLogsContext(ctx context.Context, payload WebhookPayload) string {
	service := extractServiceFromAlert(payload.Alert, payload.Message, payload.Data)
	if service == "" {
		return ""
	}

	now := time.Now()
	results, err := s.logsClient.GetContext(ctx, service, now.Add(-logsContextWindow), now, logsContextLimit)
	if err != nil {
		log.Printf("[inspector] failed to fetch log context for service %q: %v", service, err)
		return ""
	}
	if len(results) == 0 {
		return ""
	}

	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "[%s] %s: %s\n", r.Timestamp.Format(time.RFC3339), r.Severity, r.Body)
	}
	return b.String()
}

// enqueue is the shared logic for Inspect and InspectByWebhook.
func (s *service) enqueue(ctx context.Context, trigger report.TriggerType, query, contextStr string) (*report.Report, error) {
	refs := s.dsRefs()
	if len(refs) == 0 {
		return nil, errs.New(errs.ErrInvalidInput, "no datasources configured")
	}

	// Dedup: check for an active report with the same query
	existing, err := s.reportRepo.FindActiveByQuery(ctx, string(trigger), query, time.Now().Add(-s.dedupWindow))
	if err != nil {
		return nil, errs.Wrap(errs.ErrInternal, "dedup check failed", err)
	}
	if existing != nil {
		return nil, errs.New(errs.ErrConflict,
			fmt.Sprintf("a similar analysis is already in progress (report %s)", existing.ID))
	}

	triggerID := uuid.New().String()
	rpt := &report.Report{
		ID:        uuid.New().String(),
		Trigger:   trigger,
		TriggerID: triggerID,
		Status:    report.StatusQueued,
		Severity:  report.SeverityInfo,
		Query:     query,
		CreatedAt: time.Now(),
	}

	if err := s.reportRepo.Create(ctx, rpt); err != nil {
		return nil, errs.Wrap(errs.ErrInternal, "failed to create report", err)
	}

	input := &analyzer.AnalysisInput{
		Trigger:     string(trigger),
		TriggerID:   triggerID,
		Query:       query,
		Context:     contextStr,
		Datasources: refs,
	}
	job := &analysisJob{
		report: rpt,
		input:  input,
	}

	select {
	case s.queue <- job:
		log.Printf("[inspector] report %s queued (queue: %d/%d)", rpt.ID, len(s.queue), s.queueSize)
		return rpt, nil
	default:
		// Queue is full — update report to failed and reject
		failedReport := rpt.Clone()
		failedReport.Status = report.StatusFailed
		failedReport.Severity = report.SeverityInfo
		failedReport.Summary = "Analysis queue is full, request rejected"
		failedReport.Iterations = 0
		if err := s.reportRepo.Update(ctx, &failedReport); err != nil {
			log.Printf("[inspector] failed to update report %s: %v", rpt.ID, err)
		}
		return nil, errs.New(errs.ErrRateLimit, "analysis queue is full, try again later")
	}
}

func (s *service) GetReport(ctx context.Context, id string) (*report.Report, error) {
	rpt, err := s.reportRepo.GetByID(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, errs.Wrap(errs.ErrNotFound, "report not found", err)
		}
		return nil, errs.Wrap(errs.ErrInternal, "failed to get report", err)
	}
	return rpt, nil
}

func (s *service) ListReports(ctx context.Context, limit int) ([]*report.Report, error) {
	reports, err := s.reportRepo.List(ctx, limit)
	if err != nil {
		return nil, errs.Wrap(errs.ErrInternal, "failed to list reports", err)
	}
	return reports, nil
}

func (s *service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}

	// Wait for workers to finish current jobs and exit.
	// Workers read from queue with select { case job := <-s.queue: }
	// After ctx is cancelled, they finish their current job and exit.
	s.wg.Wait()

	// Drain any remaining queued jobs and mark them as failed
	// so they don't stay orphaned in DB as StatusQueued.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer drainCancel()
	drained := 0
loop:
	for {
		select {
		case job := <-s.queue:
			if job == nil {
				break loop
			}
			failedReport := job.report.Clone()
			failedReport.Status = report.StatusFailed
			failedReport.Summary = "Analysis queue drained during shutdown"
			if err := s.reportRepo.Update(drainCtx, &failedReport); err != nil {
				log.Printf("[inspector] failed to update report %s on drain: %v", job.report.ID, err)
			}
			drained++
		default:
			// Queue is empty — nothing more to drain.
			break loop
		}
	}
	if drained > 0 {
		log.Printf("[inspector] drained %d queued reports on shutdown", drained)
	}
	log.Printf("[inspector] all workers stopped")
}
