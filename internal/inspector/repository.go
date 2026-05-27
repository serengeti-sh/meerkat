package inspector

import (
	"context"
	"time"
)

// ReportRepository stores and retrieves reports.
type ReportRepository interface {
	Create(ctx context.Context, report *Report) error
	Update(ctx context.Context, report *Report) error
	GetByID(ctx context.Context, id string) (*Report, error)
	List(ctx context.Context, limit int) ([]*Report, error)
	FindActiveByQuery(ctx context.Context, trigger string, query string, since time.Time) (*Report, error)
}
