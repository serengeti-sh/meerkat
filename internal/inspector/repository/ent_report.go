package repository

import (
	"context"
	"time"

	"github.com/mandacode-labs/inspector/ent"
	entReport "github.com/mandacode-labs/inspector/ent/report"
	"github.com/mandacode-labs/inspector/internal/inspector"
)

type entReportRepository struct {
	client *ent.Client
}

func NewRepository(client *ent.Client) inspector.ReportRepository {
	return &entReportRepository{client: client}
}

func (r *entReportRepository) Create(ctx context.Context, report *inspector.Report) error {
	_, err := r.client.Report.Create().
		SetID(report.ID()).
		SetTrigger(entReport.Trigger(report.Trigger())).
		SetTriggerID(report.TriggerID()).
		SetStatus(entReport.Status(string(report.Status()))).
		SetSeverity(entReport.Severity(string(report.Severity()))).
		SetSummary(report.Summary()).
		SetDetail(report.Detail()).
		SetQuery(report.Query()).
		SetDatasources(report.Datasources()).
		SetIterations(report.Iterations()).
		Save(ctx)
	return err
}

func (r *entReportRepository) Update(ctx context.Context, report *inspector.Report) error {
	return r.client.Report.UpdateOneID(report.ID()).
		SetStatus(entReport.Status(string(report.Status()))).
		SetSeverity(entReport.Severity(string(report.Severity()))).
		SetSummary(report.Summary()).
		SetDetail(report.Detail()).
		SetQuery(report.Query()).
		SetDatasources(report.Datasources()).
		SetIterations(report.Iterations()).
		Exec(ctx)
}

func (r *entReportRepository) GetByID(ctx context.Context, id string) (*inspector.Report, error) {
	ds, err := r.client.Report.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return entToReport(ds)
}

func (r *entReportRepository) List(ctx context.Context, limit int) ([]*inspector.Report, error) {
	if limit <= 0 {
		limit = 50
	}

	list, err := r.client.Report.Query().
		Order(ent.Desc(entReport.FieldCreateTime)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*inspector.Report, 0, len(list))
	for _, r := range list {
		report, err := entToReport(r)
		if err != nil {
			return nil, err
		}
		result = append(result, report)
	}
	return result, nil
}

// FindActiveByQuery finds a pending or running report with the same trigger and query,
// created within the given time window.
func (r *entReportRepository) FindActiveByQuery(ctx context.Context, trigger string, query string, since time.Time) (*inspector.Report, error) {
	ds, err := r.client.Report.Query().
		Where(
			entReport.TriggerEQ(entReport.Trigger(trigger)),
			entReport.QueryEQ(query),
			entReport.StatusIn(entReport.StatusPending, entReport.StatusRunning),
			entReport.CreateTimeGTE(since),
		).
		Order(ent.Desc(entReport.FieldCreateTime)).
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return entToReport(ds)
}

func entToReport(r *ent.Report) (*inspector.Report, error) {
	return inspector.NewReport(
		r.ID,
		string(r.Trigger),
		r.TriggerID,
		inspector.Status(r.Status),
		inspector.Severity(r.Severity),
		r.Summary,
		r.Detail,
		r.Query,
		r.Datasources,
		r.Iterations,
		r.CreateTime,
	), nil
}
