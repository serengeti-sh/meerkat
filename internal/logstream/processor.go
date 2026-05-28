package logstream

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/serengeti-sh/meerkat/internal/rag"
)

const (
	defaultIngestWorkers = 10
	ingestQueueFactor    = 2
)

// Processor consumes log entries from a Connector, indexes them into RAG,
// and optionally triggers analysis when thresholds are breached.
type Processor struct {
	connector  *Connector
	ragSvc     rag.Service
	windowSize time.Duration
	threshold  int
	workers    int
	ingestCh   chan rag.LogEntry
}

// NewProcessor creates a stream processor.
func NewProcessor(conn *Connector, ragSvc rag.Service, windowSize time.Duration, threshold int) *Processor {
	return &Processor{
		connector:  conn,
		ragSvc:     ragSvc,
		windowSize: windowSize,
		threshold:  threshold,
		workers:    defaultIngestWorkers,
		ingestCh:   make(chan rag.LogEntry, defaultIngestWorkers*ingestQueueFactor),
	}
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

	return p.connector.Subscribe(ctx, query, func(entry rag.LogEntry) {
		window.Add(entry.Timestamp)

		select {
		case p.ingestCh <- entry:
		default:
			log.Printf("[stream] ingest queue full, dropping entry %s", entry.ID)
		}

		// Check threshold.
		if window.Count() >= p.threshold {
			log.Printf("[stream] threshold breached: %d entries in %v", window.Count(), p.windowSize)
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

func (p *Processor) indexEntry(ctx context.Context, entry rag.LogEntry) {
	_, err := p.ragSvc.Ingest(ctx, []rag.LogEntry{entry})
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
