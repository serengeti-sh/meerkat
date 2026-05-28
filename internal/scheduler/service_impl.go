package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/inspector"
	"github.com/serengeti-sh/meerkat/internal/report"
)

// Inspector is the subset of inspector.Service that the scheduler requires.
// Defined locally so the scheduler depends only on what it uses.
type Inspector interface {
	Inspect(ctx context.Context, req inspector.InspectRequest) (*report.Report, error)
}

type service struct {
	inspectorSvc Inspector
	jobs         []Job
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	startOnce    sync.Once
}

var _ Service = (*service)(nil)

// NewService creates a scheduler from configuration.
func NewService(inspectorSvc Inspector, cfg *config.Config) *service {
	var jobs []Job
	for _, j := range cfg.Scheduler.Jobs {
		d, err := time.ParseDuration(j.Interval)
		if err != nil {
			log.Printf("[scheduler] skipping job %q: invalid interval %q: %v", j.Name, j.Interval, err)
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
	}
}

func (s *service) Start(ctx context.Context) error {
	s.startOnce.Do(func() {
		ctx, s.cancel = context.WithCancel(ctx)

		for _, job := range s.jobs {
			s.wg.Add(1)
			go func(j Job) {
				defer s.wg.Done()
				log.Printf("[scheduler] starting job %q (interval: %s)", j.Name, j.Interval)

				timer := time.NewTimer(j.Interval)
				defer timer.Stop()

				for {
					select {
					case <-ctx.Done():
						log.Printf("[scheduler] stopping job %q", j.Name)
						return
					case <-timer.C:
						log.Printf("[scheduler] running job %q", j.Name)
						rpt, err := s.inspectorSvc.Inspect(ctx, inspector.InspectRequest{
							MetricQuery: j.MetricQuery,
							LogQuery:    j.LogQuery,
							Query:       "Scheduled inspection: " + j.Name,
						})
						if err != nil {
							log.Printf("[scheduler] job %q failed: %v", j.Name, err)
						}
						if err == nil {
							log.Printf("[scheduler] job %q completed: severity=%s summary=%s", j.Name, rpt.Severity(), rpt.Summary())
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
