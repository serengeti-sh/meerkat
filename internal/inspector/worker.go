package inspector

import (
	"context"
	"fmt"
	"log"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/reporter"
)

func (s *service) worker(id int, ctx context.Context) {
	defer s.wg.Done()
	log.Printf("[inspector] worker %d started", id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[inspector] worker %d shutting down", id)
			return
		case job := <-s.queue:
			if job == nil {
				return
			}
			s.runAnalysis(ctx, job.report, job.input)
		}
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
			Trigger:     string(finalReport.Trigger()),
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
