package inspector

import (
	"context"
	"fmt"
	"log"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/report"
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
func (s *service) runAnalysis(ctx context.Context, rpt *report.Report, input *analyzer.AnalysisInput) {
	// Update status to running
	runningReport := rpt.Clone(report.WithStatus(report.StatusRunning))
	if err := s.reportRepo.Update(ctx, runningReport); err != nil {
		log.Printf("[meerkat] failed to update report %s to running: %v", rpt.ID(), err)
	}

	// Run the agent loop
	result, err := s.analyzerSvc.Analyze(ctx, input)

	var finalReport *report.Report
	if err != nil {
		log.Printf("[meerkat] analysis failed for report %s: %v", rpt.ID(), err)
		finalReport = rpt.Clone(
			report.WithStatus(report.StatusFailed),
			report.WithSeverity(report.SeverityInfo),
			report.WithSummary(fmt.Sprintf("Analysis failed: %v", err)),
			report.WithIterations(0),
		)
	} else {
		finalReport = rpt.Clone(
			report.WithStatus(report.StatusCompleted),
			report.WithSeverity(result.Severity),
			report.WithSummary(result.Summary),
			report.WithDetail(result.Detail),
			report.WithDatasources(result.Datasources),
			report.WithIterations(result.Iterations),
		)
	}

	// Save final result
	if err := s.reportRepo.Update(ctx, finalReport); err != nil {
		log.Printf("[meerkat] failed to update report %s: %v", rpt.ID(), err)
	}

	// Send to reporter channels
	if finalReport.Status() == report.StatusCompleted {
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
			log.Printf("[meerkat] failed to send report %s: %v", rpt.ID(), err)
		}
	}

	log.Printf("[meerkat] report %s completed: status=%s severity=%s iterations=%d",
		finalReport.ID(), finalReport.Status(), finalReport.Severity(), finalReport.Iterations())
}
