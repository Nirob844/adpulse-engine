package service

import (
	"context"
	"testing"
	"time"

	"github.com/nirob844/mini-ad-server/internal/model"
)

type mockTrackingEventRepo struct {
	events []*model.AdEvent
}

func (m *mockTrackingEventRepo) RecordEvent(ctx context.Context, campaignID, creativeID int64, eventType, ip, userAgent string) (*model.AdEvent, error) {
	e := &model.AdEvent{
		ID:         int64(len(m.events) + 1),
		CampaignID: campaignID,
		CreativeID: creativeID,
		Type:       eventType,
		IP:         ip,
		UserAgent:  userAgent,
		CreatedAt:  time.Now(),
	}
	m.events = append(m.events, e)
	return e, nil
}

type mockTrackingCache struct {
	incrCalls int
}

func (m *mockTrackingCache) IncrImpression(ctx context.Context, ip string, creativeID int64, campaignID int64, date string) error {
	m.incrCalls++
	return nil
}

func TestTrackingService_TrackImpression(t *testing.T) {
	crRepo := newMockCreativeRepo()
	cr, _ := crRepo.Create(context.Background(), 1, 300, 250, "https://placehold.co/300x250.png", "https://example.com/target")

	evRepo := &mockTrackingEventRepo{}
	cache := &mockTrackingCache{}
	svc := NewTrackingService(crRepo, evRepo, cache)

	// 1. Success
	err := svc.TrackImpression(context.Background(), cr.ID, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(evRepo.events) != 1 {
		t.Fatalf("expected 1 event recorded, got %d", len(evRepo.events))
	}
	if evRepo.events[0].Type != model.EventTypeImpression {
		t.Errorf("expected event type 'impression', got %s", evRepo.events[0].Type)
	}
	if cache.incrCalls != 1 {
		t.Errorf("expected 1 cache incr call, got %d", cache.incrCalls)
	}

	// 2. Unknown creative
	err = svc.TrackImpression(context.Background(), 999999, "127.0.0.1", "TestAgent")
	if err != ErrCreativeNotFound {
		t.Errorf("expected ErrCreativeNotFound, got %v", err)
	}

	// 3. Invalid ID
	err = svc.TrackImpression(context.Background(), -5, "127.0.0.1", "TestAgent")
	if err != ErrInvalidCreativeID {
		t.Errorf("expected ErrInvalidCreativeID, got %v", err)
	}
}

func TestTrackingService_TrackClick(t *testing.T) {
	crRepo := newMockCreativeRepo()
	cr, _ := crRepo.Create(context.Background(), 1, 300, 250, "https://placehold.co/300x250.png", "https://example.com/target-product")

	evRepo := &mockTrackingEventRepo{}
	cache := &mockTrackingCache{}
	svc := NewTrackingService(crRepo, evRepo, cache)

	// 1. Success
	targetURL, err := svc.TrackClick(context.Background(), cr.ID, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if targetURL != "https://example.com/target-product" {
		t.Errorf("expected target URL 'https://example.com/target-product', got '%s'", targetURL)
	}
	if len(evRepo.events) != 1 {
		t.Fatalf("expected 1 event recorded, got %d", len(evRepo.events))
	}
	if evRepo.events[0].Type != model.EventTypeClick {
		t.Errorf("expected event type 'click', got %s", evRepo.events[0].Type)
	}

	// 2. Unknown creative
	_, err = svc.TrackClick(context.Background(), 999999, "127.0.0.1", "TestAgent")
	if err != ErrCreativeNotFound {
		t.Errorf("expected ErrCreativeNotFound, got %v", err)
	}

	// 3. Invalid ID
	_, err = svc.TrackClick(context.Background(), 0, "127.0.0.1", "TestAgent")
	if err != ErrInvalidCreativeID {
		t.Errorf("expected ErrInvalidCreativeID, got %v", err)
	}
}
