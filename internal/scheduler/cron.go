package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/inspector"
)

type Service interface {
	Start(ctx context.Context) error
	Stop()
}

type Job struct {
	Name        string
	Interval    time.Duration
	MetricQuery string
	LogQuery    string
}

type cronScheduler struct {
	inspectorSvc inspector.Service
	jobs         []Job
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

var _ Service = (*cronScheduler)(nil)

func NewCronScheduler(inspectorSvc inspector.Service, cfg *config.Config) Service {
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

	return &cronScheduler{
		inspectorSvc: inspectorSvc,
		jobs:         jobs,
	}
}

func (s *cronScheduler) Start(ctx context.Context) error {
	ctx, s.cancel = context.WithCancel(ctx)

	for _, job := range s.jobs {
		s.wg.Add(1)
		go func(j Job) {
			defer s.wg.Done()
			log.Printf("[scheduler] starting job %q (interval: %s)", j.Name, j.Interval)

			ticker := time.NewTicker(j.Interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					log.Printf("[scheduler] stopping job %q", j.Name)
					return
				case <-ticker.C:
					log.Printf("[scheduler] running job %q", j.Name)
					report, err := s.inspectorSvc.Inspect(ctx, inspector.InspectRequest{
						MetricQuery: j.MetricQuery,
						LogQuery:    j.LogQuery,
						Query:       "Scheduled inspection: " + j.Name,
					})
					if err != nil {
						log.Printf("[scheduler] job %q failed: %v", j.Name, err)
						continue
					}
					log.Printf("[scheduler] job %q completed: severity=%s summary=%s", j.Name, report.Severity(), report.Summary())
				}
			}
		}(job)
	}

	return nil
}

func (s *cronScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}
