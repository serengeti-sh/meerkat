package logstream

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/serengeti-sh/meerkat/internal/meerkatlogs"
)

const (
	defaultIngestWorkers = 10
	ingestQueueFactor    = 2
)

// OnThresholdBreached is called when the log rate threshold is breached.
// The service parameter identifies the source service that triggered the breach.
type OnThresholdBreached func(service string)

// Processor consumes log entries from a Connector, indexes them into MeerkatLogs,
// and optionally triggers analysis when thresholds are breached.
type Processor struct {
	connector           *Connector
	logsSvc             meerkatlogs.Service
	onThresholdBreached OnThresholdBreached
	windowSize          time.Duration
	threshold           int
	workers             int
	ingestCh            chan meerkatlogs.LogEntry
}

// NewProcessor creates a stream processor.
func NewProcessor(conn *Connector, logsSvc meerkatlogs.Service, windowSize time.Duration, threshold int) *Processor {
	return &Processor{
		connector:  conn,
		logsSvc:    logsSvc,
		windowSize: windowSize,
		threshold:  threshold,
		workers:    defaultIngestWorkers,
		ingestCh:   make(chan meerkatlogs.LogEntry, defaultIngestWorkers*ingestQueueFactor),
	}
}

// WithOnThresholdBreached sets the callback invoked when the log rate threshold
// is breached. Use this to trigger an analyzer inspection (e.g. via inspector.Service).
func (p *Processor) WithOnThresholdBreached(fn OnThresholdBreached) *Processor {
	p.onThresholdBreached = fn
	return p
}

// Run starts processing the stream until the context is cancelled.
func (p *Processor) Run(ctx context.Context, query string) error {
	window := NewSlidingWindow(p.windowSize)

	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go p.worker(ctx, &wg)
	}
	defer func() {
		close(p.ingestCh)
		wg.Wait()
	}()

	return p.connector.Subscribe(ctx, query, func(entry meerkatlogs.LogEntry) {
		window.Add(entry.Timestamp)

		select {
		case p.ingestCh <- entry:
		default:
			log.Printf("[stream] ingest queue full, dropping entry %s", entry.ID)
		}

		// Check threshold — triggers analysis when the window is full.
		if window.Count() >= p.threshold {
			log.Printf("[stream] threshold breached: %d entries in %v", window.Count(), p.windowSize)
			if p.onThresholdBreached != nil {
				// Extract service from the entry that triggered the breach
				p.onThresholdBreached(entry.Service)
			}
			window.Reset()
		}
	})
}

func (p *Processor) worker(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for entry := range p.ingestCh {
		p.indexEntry(ctx, entry)
	}
}

func (p *Processor) indexEntry(ctx context.Context, entry meerkatlogs.LogEntry) {
	_, err := p.logsSvc.Ingest(ctx, []meerkatlogs.LogEntry{entry})
	if err != nil {
		log.Printf("[stream] failed to index entry %s: %v", entry.ID, err)
	}
}

// slidingWindow counts entries within a time window.
type slidingWindow struct {
	duration time.Duration
	mu       sync.Mutex
	entries  []time.Time
}

// NewSlidingWindow creates a new sliding window with the given duration.
func NewSlidingWindow(duration time.Duration) *slidingWindow {
	return &slidingWindow{duration: duration}
}

func (w *slidingWindow) Add(t time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, t)
	w.evict(t)
}

func (w *slidingWindow) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.entries)
}

func (w *slidingWindow) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = w.entries[:0]
}

func (w *slidingWindow) evict(now time.Time) {
	cutoff := now.Add(-w.duration)
	i := 0
	for i < len(w.entries) && w.entries[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		w.entries = w.entries[i:]
	}
}
