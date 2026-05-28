package scheduler

import (
	"context"
	"time"
)

// Service runs periodic inspection jobs.
type Service interface {
	Start(ctx context.Context) error
	Stop()
}

// Job represents a scheduled inspection task.
type Job struct {
	Name        string
	Interval    time.Duration
	MetricQuery string
	LogQuery    string
}
