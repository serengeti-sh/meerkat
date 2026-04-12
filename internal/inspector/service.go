package inspector

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/serengeti-sh/meerkat/internal/analyzer"
	apperrors "github.com/serengeti-sh/meerkat/internal/errors"
	"github.com/serengeti-sh/meerkat/internal/reporter"
)

type service struct {
	analyzerSvc analyzer.AnalyzerService
	reportRepo  ReportRepository
	reporterSvc reporter.ReporterService
	registry    DatasourceRegistry
	dedupWindow time.Duration
}

// DatasourceRegistry provides access to datasource info for building refs.
type DatasourceRegistry interface {
	All() []DatasourceRef
}

// DatasourceRef is a lightweight name/type pair for datasource references.
type DatasourceRef struct {
	Name string
	Type string
}

func NewService(
	analyzerSvc analyzer.AnalyzerService,
	reportRepo ReportRepository,
	reporterSvc reporter.ReporterService,
	registry DatasourceRegistry,
	dedupWindow time.Duration,
) InspectorService {
	return &service{
		analyzerSvc: analyzerSvc,
		reportRepo:  reportRepo,
		reporterSvc: reporterSvc,
		registry:    registry,
		dedupWindow: dedupWindow,
	}
}

// Inspect creates a pending report and runs the agent in a goroutine.
func (s *service) Inspect(ctx context.Context, req InspectRequest) (*Report, apperrors.Error) {
	dsRefs := s.registry.All()
	if len(dsRefs) == 0 {
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
		StatusPending,
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

	// Run agent in background
	go s.runAnalysis(context.Background(), report, &analyzer.AnalysisInput{
		Trigger:     "manual",
		TriggerID:   triggerID,
		Query:       query,
		Datasources: toAnalyzerRefs(dsRefs),
	})

	return report, nil
}

// InspectByWebhook creates a pending report and runs the agent in a goroutine.
func (s *service) InspectByWebhook(ctx context.Context, payload WebhookPayload) (*Report, apperrors.Error) {
	dsRefs := s.registry.All()
	if len(dsRefs) == 0 {
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
		StatusPending,
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

	go s.runAnalysis(context.Background(), report, &analyzer.AnalysisInput{
		Trigger:     "webhook",
		TriggerID:   triggerID,
		Context:     fmt.Sprintf("Source: %s\nAlert: %s\nMessage: %s\nData: %s", payload.Source, payload.Alert, payload.Message, string(payload.Data)),
		Datasources: toAnalyzerRefs(dsRefs),
	})

	return report, nil
}

// runAnalysis executes the agent loop in the background.
func (s *service) runAnalysis(ctx context.Context, report *Report, input *analyzer.AnalysisInput) {
	// Update status to running
	report = NewReport(
		report.ID(), report.Trigger(), report.TriggerID(),
		StatusRunning, report.Severity(), report.Summary(),
		report.Detail(), report.Query(), report.Datasources(), report.Iterations(),
		report.CreatedAt(),
	)
	if err := s.reportRepo.Update(ctx, report); err != nil {
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

func toAnalyzerRefs(refs []DatasourceRef) []analyzer.DatasourceRef {
	out := make([]analyzer.DatasourceRef, 0, len(refs))
	for _, r := range refs {
		out = append(out, analyzer.DatasourceRef{Name: r.Name, Type: r.Type})
	}
	return out
}
