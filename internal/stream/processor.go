package stream

import (
	"context"
	"log"
	"time"

	"github.com/serengeti-sh/meerkat/internal/rag"
)

// Processor consumes log entries from a Connector, indexes them into RAG,
// and optionally triggers analysis when thresholds are breached.
type Processor struct {
	connector   *Connector
	ragSvc     rag.Service
	windowSize time.Duration
	threshold  int
}

// NewProcessor creates a stream processor.
func NewProcessor(conn *Connector, ragSvc rag.Service, windowSize time.Duration, threshold int) *Processor {
	return &Processor{
		connector:   conn,
		ragSvc:     ragSvc,
		windowSize: windowSize,
		threshold:  threshold,
	}
}

// Run starts processing the stream until the context is cancelled.
func (p *Processor) Run(ctx context.Context, query string) error {
	window := NewSlidingWindow(p.windowSize)

	return p.connector.Subscribe(ctx, query, func(entry Entry) {
		window.Add(time.UnixMilli(entry.Timestamp))

		// Index into RAG asynchronously.
		go p.indexEntry(ctx, entry)

		// Check threshold.
		if window.Count() >= p.threshold {
			log.Printf("[stream] threshold breached: %d entries in %v", window.Count(), p.windowSize)
			window.Reset()
		}
	})
}

func (p *Processor) indexEntry(ctx context.Context, entry Entry) {
	ragEntry := rag.LogEntry{
		ID:        entry.ID,
		Timestamp: time.UnixMilli(entry.Timestamp),
		Service:   entry.Service,
		Severity:  entry.Severity,
		Body:      entry.Body,
		Attributes: entry.Attributes,
	}

	_, err := p.ragSvc.Ingest(ctx, []rag.LogEntry{ragEntry})
	if err != nil {
		log.Printf("[stream] failed to index entry %s: %v", entry.ID, err)
	}
}

// slidingWindow counts entries within a time window.
type slidingWindow struct {
	duration time.Duration
	entries  []time.Time
}

// NewSlidingWindow creates a new sliding window with the given duration.
func NewSlidingWindow(duration time.Duration) *slidingWindow {
	return &slidingWindow{duration: duration}
}

func (w *slidingWindow) Add(t time.Time) {
	w.entries = append(w.entries, t)
	w.evict(t)
}

func (w *slidingWindow) Count() int {
	return len(w.entries)
}

func (w *slidingWindow) Reset() {
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
