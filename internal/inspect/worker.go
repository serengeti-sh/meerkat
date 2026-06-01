package inspect

import (
	"context"
	"fmt"

	"github.com/serengeti-sh/meerkat/internal/analyzer"
	"github.com/serengeti-sh/meerkat/internal/notify"
	"github.com/serengeti-sh/meerkat/internal/report"
)

func (s *service) worker(id int, ctx context.Context) {
	defer s.wg.Done()
	s.log.Info().Int("worker_id", id).Msg("worker started")

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Int("worker_id", id).Msg("worker shutting down")
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
	runningReport := rpt.Clone()
	runningReport.Status = report.StatusRunning
	if err := s.reportRepo.Update(ctx, &runningReport); err != nil {
		s.log.Error().Err(err).Str("report_id", rpt.ID).Msg("failed to update report to running")
	}

	// Run the agent loop
	result, err := s.analyzerSvc.Analyze(ctx, input)

	var finalReport report.Report
	if err != nil {
		s.log.Error().Err(err).Str("report_id", rpt.ID).Msg("analysis failed")
		finalReport = rpt.Clone()
		finalReport.Status = report.StatusFailed
		finalReport.Severity = report.SeverityInfo
		finalReport.Summary = fmt.Sprintf("Analysis failed: %v", err)
		finalReport.Iterations = 0
	} else {
		finalReport = rpt.Clone()
		finalReport.Status = report.StatusCompleted
		finalReport.Severity = report.Severity(result.Severity)
		finalReport.Summary = result.Summary
		finalReport.Detail = result.Detail
		finalReport.Datasources = result.Datasources
		finalReport.Iterations = result.Iterations
	}

	// Save final result
	if err := s.reportRepo.Update(ctx, &finalReport); err != nil {
		s.log.Error().Err(err).Str("report_id", rpt.ID).Msg("failed to update report")
	}

	// Send to reporter channels
	if finalReport.Status == report.StatusCompleted {
		if err := s.reporterSvc.Report(ctx, &notify.ReportData{
			ID:          finalReport.ID,
			Trigger:     string(finalReport.Trigger),
			TriggerID:   finalReport.TriggerID,
			Severity:    string(finalReport.Severity),
			Summary:     finalReport.Summary,
			Detail:      finalReport.Detail,
			Datasources: finalReport.Datasources,
			Iterations:  finalReport.Iterations,
			CreatedAt:   finalReport.CreatedAt,
		}); err != nil {
			s.log.Error().Err(err).Str("report_id", rpt.ID).Msg("failed to send report")
		}
	}

	s.log.Info().Str("report_id", finalReport.ID).Str("status", string(finalReport.Status)).Str("severity", string(finalReport.Severity)).Int("iterations", finalReport.Iterations).Msg("report completed")
}
