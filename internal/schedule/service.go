package schedule

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/inspect"
	"github.com/serengeti-sh/meerkat/internal/report"
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

// Inspector is the subset of inspect.Service that the scheduler requires.
// Defined locally so the scheduler depends only on what it uses.
type Inspector interface {
	Inspect(ctx context.Context, req inspect.Request) (*report.Report, error)
}

type service struct {
	inspectorSvc Inspector
	jobs         []Job
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	startOnce    sync.Once
	log          zerolog.Logger
}

var _ Service = (*service)(nil)

// NewService creates a scheduler from configuration.
func NewService(inspectorSvc Inspector, cfg *config.Config, log zerolog.Logger) *service {
	var jobs []Job
	for _, j := range cfg.Schedule.Jobs {
		d, err := time.ParseDuration(j.Interval)
		if err != nil {
			log.Error().Err(err).Str("job", j.Name).Str("interval", j.Interval).Msg("skipping job with invalid interval")
			continue
		}
		jobs = append(jobs, Job{
			Name:        j.Name,
			Interval:    d,
			MetricQuery: j.MetricQuery,
			LogQuery:    j.LogQuery,
		})
	}

	return &service{
		inspectorSvc: inspectorSvc,
		jobs:         jobs,
		log:          log,
	}
}

func (s *service) Start(ctx context.Context) error {
	s.startOnce.Do(func() {
		ctx, s.cancel = context.WithCancel(ctx)

		for _, job := range s.jobs {
			s.wg.Add(1)
			go func(j Job) {
				defer s.wg.Done()
				s.log.Info().Str("job", j.Name).Dur("interval", j.Interval).Msg("starting job")

				timer := time.NewTimer(j.Interval)
				defer timer.Stop()

				for {
					select {
					case <-ctx.Done():
						s.log.Info().Str("job", j.Name).Msg("stopping job")
						return
					case <-timer.C:
						s.log.Info().Str("job", j.Name).Msg("running job")
						rpt, err := s.inspectorSvc.Inspect(ctx, inspect.Request{
							MetricQuery: j.MetricQuery,
							LogQuery:    j.LogQuery,
							Query:       "Scheduled inspection: " + j.Name,
						})
						if err != nil {
							s.log.Error().Err(err).Str("job", j.Name).Msg("job failed")
						}
						if err == nil {
							s.log.Info().Str("job", j.Name).Str("severity", string(rpt.Severity)).Str("summary", rpt.Summary).Msg("job completed")
						}
						timer.Reset(j.Interval)
					}
				}
			}(job)
		}
	})

	return nil
}

func (s *service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}
