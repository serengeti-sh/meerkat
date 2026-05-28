package collector

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/serengeti-sh/meerkat/internal/config"
	"github.com/serengeti-sh/meerkat/internal/logsclient"
	"github.com/serengeti-sh/meerkat/internal/meerkatlogs"
)

const defaultFlushTimeout = 30 * time.Second

// Batcher buffers log entries and flushes them to a MeerkatLogs server.
type Batcher struct {
	logsClient    logsclient.Client
	logsService   meerkatlogs.Service
	batchSize     int
	flushInterval time.Duration
	mu            sync.Mutex
	buffer        []LogEntry
	stopCh        chan struct{}
	stopOnce      sync.Once
	wg            sync.WaitGroup
}

var _ LogSink = (*Batcher)(nil)

// NewBatcher creates a Batcher with the given configuration.
func NewBatcher(cfg *config.Config) *Batcher {
	return &Batcher{
		batchSize:     cfg.Collector.BatchSize,
		flushInterval: cfg.Collector.FlushInterval,
		stopCh:        make(chan struct{}),
	}
}

// WithLogsClient configures the batcher to send logs to a remote MeerkatLogs server via
// gRPC. The MeerkatLogs server handles deduplication via its Extractor (Drain algorithm)
// before embedding and storage.
func (b *Batcher) WithLogsClient(client logsclient.Client) *Batcher {
	b.logsClient = client
	return b
}

// WithLogsService configures the batcher to send logs to an in-process MeerkatLogs
// service. Used when the batcher and MeerkatLogs pipeline run in the same process
// (e.g. meerkatlogs server).
func (b *Batcher) WithLogsService(svc meerkatlogs.Service) *Batcher {
	b.logsService = svc
	return b
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
// Returns an error if the batcher has been stopped.
func (b *Batcher) Add(entry LogEntry) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	select {
	case <-b.stopCh:
		return fmt.Errorf("batcher is stopped")
	default:
	}

	b.buffer = append(b.buffer, entry)
	if len(b.buffer) >= b.batchSize {
		b.triggerFlushLocked()
	}
	return nil
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
	defer b.mu.Unlock()
	b.triggerFlushLocked()
}

func (b *Batcher) triggerFlushLocked() {
	if len(b.buffer) == 0 {
		return
	}
	entries := make([]LogEntry, len(b.buffer))
	copy(entries, b.buffer)
	b.buffer = b.buffer[:0]

	go b.flushAsync(entries)
}

func (b *Batcher) flushAsync(entries []LogEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultFlushTimeout)
	defer cancel()

	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if err = b.flush(ctx, entries); err == nil {
			return
		}
		log.Printf("[collector] flush attempt %d failed: %v", attempt, err)
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	log.Printf("[collector] flush failed after 3 attempts: %v", err)
}

func (b *Batcher) flush(ctx context.Context, entries []LogEntry) error {
	if b.logsService != nil {
		return b.flushToLogsService(ctx, entries)
	}
	if b.logsClient != nil {
		return b.flushToLogsClient(ctx, entries)
	}
	return fmt.Errorf("no MeerkatLogs client or service configured")
}

func (b *Batcher) flushToLogsClient(ctx context.Context, entries []LogEntry) error {
	logsEntries := make([]logsclient.LogEntry, len(entries))
	for i, e := range entries {
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		logsEntries[i] = logsclient.LogEntry{
			ID:         e.ID,
			Timestamp:  e.Timestamp,
			Service:    e.Service,
			Severity:   e.Severity,
			Body:       e.Body,
			Attributes: e.Attributes,
		}
	}

	_, err := b.logsClient.Ingest(ctx, logsEntries)
	if err != nil {
		return fmt.Errorf("ingest to MeerkatLogs client: %w", err)
	}
	return nil
}

func (b *Batcher) flushToLogsService(ctx context.Context, entries []LogEntry) error {
	logsEntries := make([]meerkatlogs.LogEntry, len(entries))
	for i, e := range entries {
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		logsEntries[i] = meerkatlogs.LogEntry{
			ID:         e.ID,
			Timestamp:  e.Timestamp,
			Service:    e.Service,
			Severity:   e.Severity,
			Body:       e.Body,
			Attributes: e.Attributes,
		}
	}

	_, err := b.logsService.Ingest(ctx, logsEntries)
	if err != nil {
		return fmt.Errorf("ingest to MeerkatLogs service: %w", err)
	}
	return nil
}
