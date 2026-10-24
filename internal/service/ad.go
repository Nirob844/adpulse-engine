package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/nirob844/adpulse-engine/internal/model"
)

// Common ad serving errors
var (
	ErrInvalidDimensions = errors.New("width and height must be positive integers")
)

// AdCreativeRepository defines creative querying interface for ad serving.
type AdCreativeRepository interface {
	GetActiveCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, error)
}

// AdEventRepository defines ad events querying interface for limit enforcement.
type AdEventRepository interface {
	CountDailyImpressions(ctx context.Context, campaignID int64, date time.Time) (int64, error)
	CountCreativeIPImpressions(ctx context.Context, creativeID int64, ip string, date time.Time) (int64, error)
}

// AdCache defines caching operations used during ad selection.
type AdCache interface {
	GetCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, bool, error)
	SetCandidates(ctx context.Context, width, height int, candidates []*model.CreativeCandidate, ttl time.Duration) error
	GetFrequencyCount(ctx context.Context, ip string, creativeID int64, date string) (int64, error)
	GetCampaignDailyImpressions(ctx context.Context, campaignID int64, date string) (int64, bool, error)
	SetCampaignDailyImpressions(ctx context.Context, campaignID int64, date string, count int64) error
}

// AdService handles ad selection, frequency capping, daily limit enforcement, and HTML tag generation.
type AdService struct {
	creativeRepo AdCreativeRepository
	eventRepo    AdEventRepository
	cache        AdCache
	freqCapMax   int
}

// NewAdService creates a new AdService instance.
func NewAdService(cr AdCreativeRepository, er AdEventRepository, cache AdCache, freqCapMax int) *AdService {
	return &AdService{
		creativeRepo: cr,
		eventRepo:    er,
		cache:        cache,
		freqCapMax:   freqCapMax,
	}
}

// ServeAd selects an active creative matching requested dimensions, applies caps, and returns an AdResponse.
// If no creative matches or all candidates are capped, it returns nil, nil (caller should respond with 204 No Content).
func (s *AdService) ServeAd(ctx context.Context, width, height int, clientIP string) (*model.AdResponse, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidDimensions
	}

	// 1. Retrieve candidates (Cache-aside with Redis)
	var candidates []*model.CreativeCandidate
	if s.cache != nil {
		if cached, hit, _ := s.cache.GetCandidates(ctx, width, height); hit {
			candidates = cached
		}
	}

	if candidates == nil {
		dbCandidates, err := s.creativeRepo.GetActiveCandidates(ctx, width, height)
		if err != nil {
			return nil, fmt.Errorf("failed to query active creative candidates: %w", err)
		}
		candidates = dbCandidates

		// Cache candidates for 60 seconds
		if s.cache != nil && len(candidates) > 0 {
			_ = s.cache.SetCandidates(ctx, width, height, candidates, 60*time.Second)
		}
	}

	if len(candidates) == 0 {
		return nil, nil // No active creative found -> 204 No Content
	}

	// 2. Filter candidates based on Campaign Daily Limits and IP Frequency Capping
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	var eligible []*model.CreativeCandidate

	for _, c := range candidates {
		// A. Check Campaign Daily Impression Limit
		if c.CampaignDailyLimit > 0 {
			var dailyImpressions int64
			cachedCount, hit := int64(0), false

			if s.cache != nil {
				cachedCount, hit, _ = s.cache.GetCampaignDailyImpressions(ctx, c.CampaignID, today)
			}

			if hit {
				dailyImpressions = cachedCount
			} else if s.eventRepo != nil {
				dbCount, err := s.eventRepo.CountDailyImpressions(ctx, c.CampaignID, now)
				if err == nil {
					dailyImpressions = dbCount
					if s.cache != nil {
						_ = s.cache.SetCampaignDailyImpressions(ctx, c.CampaignID, today, dbCount)
					}
				}
			}

			if dailyImpressions >= int64(c.CampaignDailyLimit) {
				continue // Skip: Campaign reached daily limit
			}
		}

		// B. Check IP Frequency Capping (e.g. max 3 impressions per creative per IP per day)
		if clientIP != "" && s.freqCapMax > 0 {
			var ipImpressions int64
			if s.cache != nil {
				count, _ := s.cache.GetFrequencyCount(ctx, clientIP, c.ID, today)
				ipImpressions = count
			}

			if ipImpressions == 0 && s.eventRepo != nil {
				dbCount, err := s.eventRepo.CountCreativeIPImpressions(ctx, c.ID, clientIP, now)
				if err == nil {
					ipImpressions = dbCount
				}
			}

			if ipImpressions >= int64(s.freqCapMax) {
				continue // Skip: IP frequency cap reached for this creative
			}
		}

		eligible = append(eligible, c)
	}

	if len(eligible) == 0 {
		return nil, nil // All candidates reached limit/cap -> 204 No Content
	}

	// 3. Selection (random among eligible candidates)
	chosen := eligible[rand.Intn(len(eligible))]

	// 4. Construct response with image tag and tracking URLs
	return &model.AdResponse{
		CreativeID:    chosen.ID,
		HTML:          fmt.Sprintf(`<img src="%s" width="%d" height="%d" alt="Advertisement" />`, chosen.ImageURL, chosen.Width, chosen.Height),
		ImpressionURL: fmt.Sprintf("/track/impression?c=%d", chosen.ID),
		ClickURL:      fmt.Sprintf("/track/click?c=%d", chosen.ID),
	}, nil
}
