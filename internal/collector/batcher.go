package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/embedder"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

const defaultFlushTimeout = 30 * time.Second

// Batcher buffers log entries and flushes them to the vector store.
type Batcher struct {
	embedder      embedder.Embedder
	vectorstore   vectorstore.VectorStore
	batchSize     int
	flushInterval time.Duration
	mu            sync.Mutex
	buffer        []LogEntry
	stopCh        chan struct{}
	stopOnce      sync.Once
	wg            sync.WaitGroup
}

var _ LogAdder = (*Batcher)(nil)

// NewBatcher creates a Batcher with the given configuration.
func NewBatcher(cfg *config.Config, emb embedder.Embedder, vs vectorstore.VectorStore) *Batcher {
	return &Batcher{
		embedder:      emb,
		vectorstore:   vs,
		batchSize:     cfg.Collector.BatchSize,
		flushInterval: cfg.Collector.FlushInterval,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background flush loop.
func (b *Batcher) Start() {
	b.wg.Add(1)
	go b.loop()
}

// Stop halts the background flush loop and flushes any remaining entries.
func (b *Batcher) Stop(ctx context.Context) {
	b.stopOnce.Do(func() {
		close(b.stopCh)
	})
	b.wg.Wait()
	b.mu.Lock()
	entries := make([]LogEntry, len(b.buffer))
	copy(entries, b.buffer)
	b.buffer = b.buffer[:0]
	b.mu.Unlock()
	if len(entries) > 0 {
		if err := b.flush(ctx, entries); err != nil {
			log.Printf("[collector] final flush failed: %v", err)
		}
	}
}

// Add appends a log entry to the buffer and triggers a flush if the batch is full.
func (b *Batcher) Add(entry LogEntry) {
	b.mu.Lock()
	b.buffer = append(b.buffer, entry)
	shouldFlush := len(b.buffer) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		b.triggerFlush()
	}
}

func (b *Batcher) loop() {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			b.triggerFlush()
		}
	}
}

func (b *Batcher) triggerFlush() {
	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return
	}
	entries := make([]LogEntry, len(b.buffer))
	copy(entries, b.buffer)
	b.buffer = b.buffer[:0]
	b.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), defaultFlushTimeout)
	defer cancel()

	if err := b.flush(ctx, entries); err != nil {
		log.Printf("[collector] flush failed: %v", err)
	}
}

func (b *Batcher) flush(ctx context.Context, entries []LogEntry) error {
	texts := make([]string, len(entries))
	for i, e := range entries {
		texts[i] = e.Body
	}

	vectors, err := b.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed texts: %w", err)
	}

	records := make([]vectorstore.Record, len(entries))
	for i, e := range entries {
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		records[i] = vectorstore.Record{
			ID:         e.ID,
			Vector:     vectors[i],
			Timestamp:  e.Timestamp,
			Service:    e.Service,
			Severity:   e.Severity,
			Body:       e.Body,
			Attributes: e.Attributes,
		}
	}

	return b.vectorstore.Insert(ctx, records)
}
