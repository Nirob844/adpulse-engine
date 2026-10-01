package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nirob844/mini-ad-server/internal/model"
)

// BatchEventRepo defines the bulk insertion interface for ad events.
type BatchEventRepo interface {
	BatchInsert(ctx context.Context, events []*model.AdEvent) error
}

// IngesterConfig specifies tuning options for the asynchronous event ingester.
type IngesterConfig struct {
	BatchSize     int           // Max events to buffer before flushing (default: 100)
	FlushInterval time.Duration // Max duration to wait before flushing (default: 100ms)
	QueueCapacity int           // In-memory queue channel buffer size (default: 10,000)
}

// EventIngester collects ad impression and click events asynchronously and flushes them
// to the persistent database in bulk using a background worker goroutine.
type EventIngester struct {
	repo          BatchEventRepo
	eventChan     chan *model.AdEvent
	batchSize     int
	flushInterval time.Duration

	buffer []*model.AdEvent
	mu     sync.Mutex

	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
	running atomic.Bool

	droppedCount atomic.Int64
	flushedCount atomic.Int64
}

// NewEventIngester creates and configures a new EventIngester instance.
func NewEventIngester(repo BatchEventRepo, cfg IngesterConfig) *EventIngester {
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	flushInterval := cfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = 100 * time.Millisecond
	}

	queueCapacity := cfg.QueueCapacity
	if queueCapacity <= 0 {
		queueCapacity = 10000
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &EventIngester{
		repo:          repo,
		eventChan:     make(chan *model.AdEvent, queueCapacity),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		buffer:        make([]*model.AdEvent, 0, batchSize),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start spawns the background worker goroutine.
func (ing *EventIngester) Start() {
	if ing.running.Swap(true) {
		return // already running
	}

	ing.wg.Add(1)
	go ing.worker()
}

// Enqueue adds an event to the ingestion queue.
// Returns true if successfully queued, or false if the queue is saturated.
func (ing *EventIngester) Enqueue(event *model.AdEvent) bool {
	if event == nil || !ing.running.Load() {
		return false
	}

	select {
	case ing.eventChan <- event:
		return true
	default:
		ing.droppedCount.Add(1)
		slog.Warn("Event ingestion queue saturated, dropping event", "creative_id", event.CreativeID, "type", event.Type)
		return false
	}
}

// worker is the background loop collecting and periodically flushing events.
func (ing *EventIngester) worker() {
	defer ing.wg.Done()

	ticker := time.NewTicker(ing.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ing.ctx.Done():
			ing.drainAndFlush()
			return

		case event, ok := <-ing.eventChan:
			if !ok {
				ing.drainAndFlush()
				return
			}
			ing.mu.Lock()
			ing.buffer = append(ing.buffer, event)
			needFlush := len(ing.buffer) >= ing.batchSize
			ing.mu.Unlock()

			if needFlush {
				ing.flush()
			}

		case <-ticker.C:
			ing.mu.Lock()
			hasItems := len(ing.buffer) > 0
			ing.mu.Unlock()

			if hasItems {
				ing.flush()
			}
		}
	}
}

// flush writes the currently buffered events to the database and clears the buffer.
func (ing *EventIngester) flush() {
	ing.mu.Lock()
	if len(ing.buffer) == 0 {
		ing.mu.Unlock()
		return
	}

	batch := ing.buffer
	ing.buffer = make([]*model.AdEvent, 0, ing.batchSize)
	ing.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if ing.repo != nil {
		if err := ing.repo.BatchInsert(ctx, batch); err != nil {
			slog.Error("Failed to bulk insert ad event batch", "count", len(batch), "error", err)
			return
		}
	}

	ing.flushedCount.Add(int64(len(batch)))
}

// drainAndFlush reads all remaining queued items and flushes them before exit.
func (ing *EventIngester) drainAndFlush() {
	ing.mu.Lock()
	defer ing.mu.Unlock()

	// Drain remaining events in channel
	for {
		select {
		case event, ok := <-ing.eventChan:
			if !ok {
				goto FLUSH
			}
			ing.buffer = append(ing.buffer, event)
		default:
			goto FLUSH
		}
	}

FLUSH:
	if len(ing.buffer) > 0 && ing.repo != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := ing.repo.BatchInsert(ctx, ing.buffer); err != nil {
			slog.Error("Failed to flush final event batch on shutdown", "count", len(ing.buffer), "error", err)
		} else {
			ing.flushedCount.Add(int64(len(ing.buffer)))
		}
		ing.buffer = nil
	}
}

// Stop gracefully shuts down the ingester, draining and flushing all queued events.
func (ing *EventIngester) Stop() {
	if !ing.running.Swap(false) {
		return
	}

	close(ing.eventChan)
	ing.cancel()
	ing.wg.Wait()
}

// FlushedCount returns the total number of events successfully batch inserted.
func (ing *EventIngester) FlushedCount() int64 {
	return ing.flushedCount.Load()
}

// DroppedCount returns the total number of events dropped due to queue saturation.
func (ing *EventIngester) DroppedCount() int64 {
	return ing.droppedCount.Load()
}

