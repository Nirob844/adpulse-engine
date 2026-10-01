package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nirob844/mini-ad-server/internal/model"
	"github.com/nirob844/mini-ad-server/internal/repo"
)

// TrackingCreativeRepo defines creative retrieval for tracking.
type TrackingCreativeRepo interface {
	GetByID(ctx context.Context, id int64) (*model.Creative, error)
}

// TrackingEventRepo defines event recording interface for tracking.
type TrackingEventRepo interface {
	RecordEvent(ctx context.Context, campaignID, creativeID int64, eventType, ip, userAgent string) (*model.AdEvent, error)
}

// TrackingCache defines counter increment operations in Redis.
type TrackingCache interface {
	IncrImpression(ctx context.Context, ip string, creativeID int64, campaignID int64, date string) error
}

// TrackingService orchestrates impression and click tracking and metrics updates.
type TrackingService struct {
	creativeRepo TrackingCreativeRepo
	eventRepo    TrackingEventRepo
	cache        TrackingCache
	ingester     *EventIngester
}

// NewTrackingService creates a new TrackingService with optional async event ingester.
func NewTrackingService(cr TrackingCreativeRepo, er TrackingEventRepo, cache TrackingCache, ingester ...*EventIngester) *TrackingService {
	var ing *EventIngester
	if len(ingester) > 0 {
		ing = ingester[0]
	}
	return &TrackingService{
		creativeRepo: cr,
		eventRepo:    er,
		cache:        cache,
		ingester:     ing,
	}
}

// SetIngester configures an asynchronous event ingester.
func (s *TrackingService) SetIngester(ing *EventIngester) {
	s.ingester = ing
}

// TrackImpression validates creative existence, persists the impression event, and increments daily/frequency counters.
func (s *TrackingService) TrackImpression(ctx context.Context, creativeID int64, ip, userAgent string) error {
	if creativeID <= 0 {
		return ErrInvalidCreativeID
	}

	cr, err := s.creativeRepo.GetByID(ctx, creativeID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || errors.Is(err, ErrCreativeNotFound) {
			return ErrCreativeNotFound
		}
		return fmt.Errorf("failed to get creative for impression: %w", err)
	}

	event := &model.AdEvent{
		CampaignID: cr.CampaignID,
		CreativeID: cr.ID,
		Type:       model.EventTypeImpression,
		IP:         ip,
		UserAgent:  userAgent,
		CreatedAt:  time.Now().UTC(),
	}

	// Persist event (asynchronous batch ingester with synchronous fallback)
	if s.ingester != nil {
		if !s.ingester.Enqueue(event) && s.eventRepo != nil {
			_, _ = s.eventRepo.RecordEvent(ctx, event.CampaignID, event.CreativeID, event.Type, event.IP, event.UserAgent)
		}
	} else if s.eventRepo != nil {
		_, err = s.eventRepo.RecordEvent(ctx, event.CampaignID, event.CreativeID, event.Type, event.IP, event.UserAgent)
		if err != nil {
			return fmt.Errorf("failed to record impression event: %w", err)
		}
	}

	// Increment counters in Redis (frequency cap & campaign daily limit)
	if s.cache != nil {
		today := time.Now().UTC().Format("2006-01-02")
		_ = s.cache.IncrImpression(ctx, ip, cr.ID, cr.CampaignID, today)
	}

	return nil
}

// TrackClick validates creative existence, records the click event, and returns the destination landing URL.
func (s *TrackingService) TrackClick(ctx context.Context, creativeID int64, ip, userAgent string) (string, error) {
	if creativeID <= 0 {
		return "", ErrInvalidCreativeID
	}

	cr, err := s.creativeRepo.GetByID(ctx, creativeID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) || errors.Is(err, ErrCreativeNotFound) {
			return "", ErrCreativeNotFound
		}
		return "", fmt.Errorf("failed to get creative for click: %w", err)
	}

	event := &model.AdEvent{
		CampaignID: cr.CampaignID,
		CreativeID: cr.ID,
		Type:       model.EventTypeClick,
		IP:         ip,
		UserAgent:  userAgent,
		CreatedAt:  time.Now().UTC(),
	}

	// Persist click event (asynchronous batch ingester with synchronous fallback)
	if s.ingester != nil {
		if !s.ingester.Enqueue(event) && s.eventRepo != nil {
			_, _ = s.eventRepo.RecordEvent(ctx, event.CampaignID, event.CreativeID, event.Type, event.IP, event.UserAgent)
		}
	} else if s.eventRepo != nil {
		_, err = s.eventRepo.RecordEvent(ctx, event.CampaignID, event.CreativeID, event.Type, event.IP, event.UserAgent)
		if err != nil {
			return "", fmt.Errorf("failed to record click event: %w", err)
		}
	}

	return cr.ClickURL, nil
}
