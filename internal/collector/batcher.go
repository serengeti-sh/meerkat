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
	"github.com/serengeti-sh/meerkat/internal/logsclient"
	"github.com/serengeti-sh/meerkat/internal/meerkatlogs"
	"github.com/serengeti-sh/meerkat/internal/vectorstore"
)

const defaultFlushTimeout = 30 * time.Second

// Batcher buffers log entries and flushes them to the vector store or MeerkatLogs server.
type Batcher struct {
	embedder      embedder.Model
	vectorstore   vectorstore.Store
	logsClient    logsclient.Client
	ragService    meerkatlogs.Service
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
func NewBatcher(cfg *config.Config, emb embedder.Model, vstore vectorstore.Store) *Batcher {
	return &Batcher{
		embedder:      emb,
		vectorstore:   vstore,
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

// WithLogsService configures the batcher to send logs to an in-process RAG
// service. Used when the batcher and RAG pipeline run in the same process
// (e.g. meerkatlogs server).
func (b *Batcher) WithLogsService(svc meerkatlogs.Service) *Batcher {
	b.ragService = svc
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

	if err := b.flush(ctx, entries); err != nil {
		log.Printf("[collector] flush failed: %v", err)
	}
}

func (b *Batcher) flush(ctx context.Context, entries []LogEntry) error {
	if b.ragService != nil {
		return b.flushToRAGService(ctx, entries)
	}
	if b.logsClient != nil {
		return b.flushToRAGClient(ctx, entries)
	}
	return b.flushToVectorStore(ctx, entries)
}

func (b *Batcher) flushToRAGClient(ctx context.Context, entries []LogEntry) error {
	ragEntries := make([]logsclient.LogEntry, len(entries))
	for i, e := range entries {
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		ragEntries[i] = logsclient.LogEntry{
			ID:         e.ID,
			Timestamp:  e.Timestamp,
			Service:    e.Service,
			Severity:   e.Severity,
			Body:       e.Body,
			Attributes: e.Attributes,
		}
	}

	_, err := b.logsClient.Ingest(ctx, ragEntries)
	if err != nil {
		return fmt.Errorf("ingest to MeerkatLogs client: %w", err)
	}
	return nil
}

func (b *Batcher) flushToRAGService(ctx context.Context, entries []LogEntry) error {
	ragEntries := make([]meerkatlogs.LogEntry, len(entries))
	for i, e := range entries {
		if e.ID == "" {
			e.ID = uuid.New().String()
		}
		ragEntries[i] = meerkatlogs.LogEntry{
			ID:         e.ID,
			Timestamp:  e.Timestamp,
			Service:    e.Service,
			Severity:   e.Severity,
			Body:       e.Body,
			Attributes: e.Attributes,
		}
	}

	_, err := b.ragService.Ingest(ctx, ragEntries)
	if err != nil {
		return fmt.Errorf("ingest to MeerkatLogs service: %w", err)
	}
	return nil
}

func (b *Batcher) flushToVectorStore(ctx context.Context, entries []LogEntry) error {
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
