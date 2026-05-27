package inspector

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/serengeti-sh/meerkat/internal/analyzer"
	apperrors "github.com/serengeti-sh/meerkat/internal/apperrors"
	"github.com/serengeti-sh/meerkat/internal/reporter"
	"github.com/serengeti-sh/meerkat/pkg/ragclient"
)

const (
	defaultQueueSize   = 1000
	defaultWorkerCount = 10
	ragContextWindow   = 15 * time.Minute
	ragContextLimit    = 20
)

// ServiceOption configures the inspector service.
type ServiceOption func(*service)

// WithRAGClient sets the RAG client for online log retrieval.
func WithRAGClient(client ragclient.Client) ServiceOption {
	return func(s *service) {
		s.ragClient = client
	}
}

// DatasourceRefs provides the current list of datasource references for analysis.
type DatasourceRefs func() []analyzer.DatasourceRef

type service struct {
	analyzerSvc analyzer.AnalyzerService
	reportRepo  ReportRepository
	reporterSvc reporter.ReporterService
	ragClient   ragclient.Client
	dsRefs      DatasourceRefs
	dedupWindow time.Duration
	queueSize   int
	workerCount int
	queue       chan *analysisJob
	wg          sync.WaitGroup
	cancel      context.CancelFunc
}

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
	opts ...ServiceOption,
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

	for _, opt := range opts {
		opt(s)
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

	// Online Retrieval: fetch recent log context from RAG
	if s.ragClient != nil {
		ragCtx := s.fetchRAGContext(ctx, payload)
		if ragCtx != "" {
			contextStr += "\n\n=== Recent Log Context ===\n" + ragCtx
		}
	}

	return s.enqueue(ctx, TriggerWebhook, payload.Alert, contextStr)
}

// fetchRAGContext extracts a service name from the webhook payload and queries
// the RAG index for recent log entries. Returns empty string on error.
func (s *service) fetchRAGContext(ctx context.Context, payload WebhookPayload) string {
	service := extractServiceFromAlert(payload.Alert, payload.Message)
	if service == "" {
		return ""
	}

	now := time.Now()
	results, err := s.ragClient.GetContext(ctx, service, now.Add(-ragContextWindow), now, ragContextLimit)
	if err != nil {
		log.Printf("[inspector] failed to fetch RAG context for service %q: %v", service, err)
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

// extractServiceFromAlert attempts to find a service name in the alert or message.
func extractServiceFromAlert(alert, message string) string {
	// Simple heuristic: look for common service name patterns
	// In production this would be more sophisticated (regex, known labels, etc.)
	for _, text := range []string{alert, message} {
		if text == "" {
			continue
		}
		// Look for "service=" or "service:" patterns
		if idx := strings.Index(text, "service="); idx != -1 {
			start := idx + len("service=")
			end := strings.IndexAny(text[start:], " \t\n,;}")
			if end == -1 {
				return text[start:]
			}
			return text[start : start+end]
		}
		if idx := strings.Index(text, "service:"); idx != -1 {
			start := idx + len("service:")
			end := strings.IndexAny(text[start:], " \t\n,;}")
			if end == -1 {
				return strings.TrimSpace(text[start:])
			}
			return strings.TrimSpace(text[start : start+end])
		}
	}
	return ""
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
