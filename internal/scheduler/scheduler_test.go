package scheduler_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	inspectorMocks "github.com/serengeti-sh/meerkat/internal/inspector/mocks"
	"github.com/serengeti-sh/meerkat/internal/scheduler"
)

func TestNewCronScheduler_InvalidInterval(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	cfg := &config.Config{
		Scheduler: config.SchedulerConfig{
			Enabled: false,
			Jobs: []config.SchedulerJobConfig{
				{Name: "bad-job", Interval: "invalid", MetricQuery: "up"},
			},
		},
	}

	s := scheduler.NewCronScheduler(mockSvc, cfg)
	assert.NotNil(t, s)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := s.Start(ctx)
	assert.NoError(t, err)
	s.Stop()
}

func TestCronScheduler_StartStop(t *testing.T) {
	mockSvc := inspectorMocks.NewServiceMock(t)
	mockSvc.On("Inspect", mock.Anything, inspector.InspectRequest{
		MetricQuery: "up",
		LogQuery:    "",
		Query:       "Scheduled inspection: test-job",
	}).Return(
		inspector.NewReport("r-1", inspector.TriggerScheduled, "", inspector.StatusCompleted, inspector.SeverityInfo, "ok", "", "", []string{}, 1, time.Now()),
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

	s := scheduler.NewCronScheduler(mockSvc, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := s.Start(ctx)
	assert.NoError(t, err)

	time.Sleep(300 * time.Millisecond)
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

	s := scheduler.NewCronScheduler(mockSvc, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := s.Start(ctx)
	assert.NoError(t, err)
	s.Stop()
}
