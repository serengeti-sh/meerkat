package schedule_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/inspect"
	inspectorMocks "github.com/serengeti-sh/meerkat/internal/inspect/mocks"
	"github.com/serengeti-sh/meerkat/internal/report"
	"github.com/serengeti-sh/meerkat/internal/schedule"
)

func TestNewService_InvalidInterval(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled: false,
			Jobs: []config.SchedulerJobConfig{
				{Name: "bad-job", Interval: "invalid", MetricQuery: "up"},
			},
		},
	}

	s := schedule.NewService(mockSvc, cfg)
	assert.NotNil(t, s)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := s.Start(ctx)
	assert.NoError(t, err)
	s.Stop()
}

func TestCronScheduler_StartStop(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("Inspect", mock.Anything, inspect.Request{
		MetricQuery: "up",
		LogQuery:    "",
		Query:       "Scheduled inspection: test-job",
	}).Return(
		&report.Report{ID: "r-1", Trigger: report.TriggerScheduled, Status: report.StatusCompleted, Severity: report.SeverityInfo, Summary: "ok", Datasources: []string{}, Iterations: 1, CreatedAt: time.Now()},
		nil,
	)

	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled: false,
			Jobs: []config.SchedulerJobConfig{
				{Name: "test-job", Interval: "100ms", MetricQuery: "up"},
			},
		},
	}

	s := schedule.NewService(mockSvc, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := s.Start(ctx)
	assert.NoError(t, err)

	time.Sleep(150 * time.Millisecond)
	s.Stop()

	mockSvc.AssertExpectations(t)
}

func TestCronScheduler_StartStop_NoJobs(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled: false,
			Jobs:    []config.SchedulerJobConfig{},
		},
	}

	s := schedule.NewService(mockSvc, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := s.Start(ctx)
	assert.NoError(t, err)
	s.Stop()
}

