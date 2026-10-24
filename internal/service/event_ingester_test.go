package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nirob844/adpulse-engine/internal/model"
)

type mockBatchEventRepo struct {
	mu            sync.Mutex
	batches       [][]*model.AdEvent
	totalInserted int
}

func (m *mockBatchEventRepo) BatchInsert(ctx context.Context, events []*model.AdEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]*model.AdEvent, len(events))
	copy(copied, events)
	m.batches = append(m.batches, copied)
	m.totalInserted += len(events)
	return nil
}

func (m *mockBatchEventRepo) GetTotal() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalInserted
}

func (m *mockBatchEventRepo) GetBatchCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.batches)
}

func TestEventIngester_BatchSizeThreshold(t *testing.T) {
	mockRepo := &mockBatchEventRepo{}
	cfg := IngesterConfig{
		BatchSize:     5,
		FlushInterval: 1 * time.Second, // long interval so size threshold triggers flush
		QueueCapacity: 50,
	}

	ing := NewEventIngester(mockRepo, cfg)
	ing.Start()
	defer ing.Stop()

	// Enqueue 5 events (matching batch size)
	for i := int64(1); i <= 5; i++ {
		ok := ing.Enqueue(&model.AdEvent{
			CampaignID: 1,
			CreativeID: i,
			Type:       model.EventTypeImpression,
		})
		if !ok {
			t.Fatalf("expected Enqueue to succeed for event %d", i)
		}
	}

	// Wait briefly for worker to process batch threshold
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if mockRepo.GetTotal() >= 5 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if mockRepo.GetTotal() != 5 {
		t.Errorf("expected 5 events flushed by batch threshold, got %d", mockRepo.GetTotal())
	}
}

func TestEventIngester_FlushIntervalTicker(t *testing.T) {
	mockRepo := &mockBatchEventRepo{}
	cfg := IngesterConfig{
		BatchSize:     100, // high threshold
		FlushInterval: 50 * time.Millisecond,
		QueueCapacity: 50,
	}

	ing := NewEventIngester(mockRepo, cfg)
	ing.Start()
	defer ing.Stop()

	// Enqueue only 2 events (well below batch size of 100)
	ing.Enqueue(&model.AdEvent{CampaignID: 1, CreativeID: 1, Type: model.EventTypeImpression})
	ing.Enqueue(&model.AdEvent{CampaignID: 1, CreativeID: 2, Type: model.EventTypeClick})

	// Wait for ticker to fire
	time.Sleep(120 * time.Millisecond)

	if mockRepo.GetTotal() != 2 {
		t.Errorf("expected 2 events flushed by ticker, got %d", mockRepo.GetTotal())
	}
}

func TestEventIngester_GracefulStop(t *testing.T) {
	mockRepo := &mockBatchEventRepo{}
	cfg := IngesterConfig{
		BatchSize:     1000,
		FlushInterval: 10 * time.Second,
		QueueCapacity: 50,
	}

	ing := NewEventIngester(mockRepo, cfg)
	ing.Start()

	// Enqueue 7 events
	for i := int64(1); i <= 7; i++ {
		ing.Enqueue(&model.AdEvent{CampaignID: 2, CreativeID: i, Type: model.EventTypeImpression})
	}

	// Immediate stop should drain and flush all 7 events
	ing.Stop()

	if mockRepo.GetTotal() != 7 {
		t.Errorf("expected all 7 events flushed on Stop, got %d", mockRepo.GetTotal())
	}
	if ing.FlushedCount() != 7 {
		t.Errorf("expected FlushedCount 7, got %d", ing.FlushedCount())
	}
}

func TestEventIngester_EnqueueNotRunning(t *testing.T) {
	mockRepo := &mockBatchEventRepo{}
	ing := NewEventIngester(mockRepo, IngesterConfig{})

	// Not started yet
	ok := ing.Enqueue(&model.AdEvent{CampaignID: 1})
	if ok {
		t.Errorf("expected Enqueue to return false when ingester is not running")
	}
}

func TestEventIngester_QueueSaturation(t *testing.T) {
	mockRepo := &mockBatchEventRepo{}
	cfg := IngesterConfig{
		BatchSize:     1000,
		FlushInterval: 10 * time.Second,
		QueueCapacity: 2, // tiny capacity to test drop
	}

	ing := NewEventIngester(mockRepo, cfg)
	// Don't start worker to fill channel
	ing.running.Store(true)

	ing.Enqueue(&model.AdEvent{CampaignID: 1})
	ing.Enqueue(&model.AdEvent{CampaignID: 2})

	// 3rd item should overflow
	ok := ing.Enqueue(&model.AdEvent{CampaignID: 3})
	if ok {
		t.Errorf("expected Enqueue to return false on full channel")
	}
	if ing.DroppedCount() != 1 {
		t.Errorf("expected DroppedCount 1, got %d", ing.DroppedCount())
	}
}
