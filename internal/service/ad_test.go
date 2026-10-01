package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nirob844/mini-ad-server/internal/model"
)

type mockAdCache struct {
	candidates map[string][]*model.CreativeCandidate
	freqCounts map[string]int64
	dailyCounts map[string]int64
}

func newMockAdCache() *mockAdCache {
	return &mockAdCache{
		candidates:  make(map[string][]*model.CreativeCandidate),
		freqCounts:  make(map[string]int64),
		dailyCounts: make(map[string]int64),
	}
}

func (m *mockAdCache) GetCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, bool, error) {
	key := fmt.Sprintf("%dx%d", width, height)
	val, ok := m.candidates[key]
	return val, ok, nil
}

func (m *mockAdCache) SetCandidates(ctx context.Context, width, height int, candidates []*model.CreativeCandidate, ttl time.Duration) error {
	key := fmt.Sprintf("%dx%d", width, height)
	m.candidates[key] = candidates
	return nil
}

func (m *mockAdCache) GetFrequencyCount(ctx context.Context, ip string, creativeID int64, date string) (int64, error) {
	key := fmt.Sprintf("%s:%d:%s", ip, creativeID, date)
	return m.freqCounts[key], nil
}

func (m *mockAdCache) GetCampaignDailyImpressions(ctx context.Context, campaignID int64, date string) (int64, bool, error) {
	key := fmt.Sprintf("%d:%s", campaignID, date)
	val, ok := m.dailyCounts[key]
	return val, ok, nil
}

func (m *mockAdCache) SetCampaignDailyImpressions(ctx context.Context, campaignID int64, date string, count int64) error {
	key := fmt.Sprintf("%d:%s", campaignID, date)
	m.dailyCounts[key] = count
	return nil
}

type mockAdCreativeRepo struct {
	candidates []*model.CreativeCandidate
}

func (m *mockAdCreativeRepo) GetActiveCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, error) {
	var result []*model.CreativeCandidate
	for _, c := range m.candidates {
		if c.Width == width && c.Height == height {
			result = append(result, c)
		}
	}
	return result, nil
}

func TestAdService_ServeAd_Success(t *testing.T) {
	crRepo := &mockAdCreativeRepo{
		candidates: []*model.CreativeCandidate{
			{
				ID:                 10,
				CampaignID:         1,
				CampaignName:       "Campaign 1",
				CampaignDailyLimit: 5000,
				Width:              300,
				Height:             250,
				ImageURL:           "https://placehold.co/300x250.png",
				ClickURL:           "https://example.com/target",
			},
		},
	}
	cache := newMockAdCache()
	svc := NewAdService(crRepo, nil, cache, 3)

	resp, err := svc.ServeAd(context.Background(), 300, 250, "192.168.1.1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected ad response, got nil")
	}

	if resp.CreativeID != 10 {
		t.Errorf("expected creative_id 10, got %d", resp.CreativeID)
	}
	if !strings.Contains(resp.HTML, `src="https://placehold.co/300x250.png"`) {
		t.Errorf("expected image src in html, got: %s", resp.HTML)
	}
	if resp.ImpressionURL != "/track/impression?c=10" {
		t.Errorf("expected /track/impression?c=10, got: %s", resp.ImpressionURL)
	}
	if resp.ClickURL != "/track/click?c=10" {
		t.Errorf("expected /track/click?c=10, got: %s", resp.ClickURL)
	}
}

func TestAdService_ServeAd_NoMatch(t *testing.T) {
	crRepo := &mockAdCreativeRepo{candidates: nil}
	cache := newMockAdCache()
	svc := NewAdService(crRepo, nil, cache, 3)

	// Non-matching size -> should return nil, nil (204 No Content)
	resp, err := svc.ServeAd(context.Background(), 468, 60, "192.168.1.1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response for non-matching size, got %+v", resp)
	}
}

func TestAdService_ServeAd_FrequencyCapExceeded(t *testing.T) {
	crRepo := &mockAdCreativeRepo{
		candidates: []*model.CreativeCandidate{
			{
				ID:                 10,
				CampaignID:         1,
				CampaignDailyLimit: 5000,
				Width:              300,
				Height:             250,
				ImageURL:           "https://placehold.co/300x250.png",
			},
		},
	}
	cache := newMockAdCache()
	today := time.Now().UTC().Format("2006-01-02")
	// Set 3 impressions already served to this IP today
	cache.freqCounts[fmt.Sprintf("192.168.1.5:10:%s", today)] = 3

	svc := NewAdService(crRepo, nil, cache, 3)

	// Request with capped IP -> should be filtered out
	resp, err := svc.ServeAd(context.Background(), 300, 250, "192.168.1.5")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response due to frequency cap, got %+v", resp)
	}

	// Request with different IP -> should succeed
	resp2, err := svc.ServeAd(context.Background(), 300, 250, "192.168.1.6")
	if err != nil || resp2 == nil {
		t.Fatalf("expected ad response for uncapped IP, got %v", err)
	}
}

func TestAdService_ServeAd_DailyLimitExceeded(t *testing.T) {
	crRepo := &mockAdCreativeRepo{
		candidates: []*model.CreativeCandidate{
			{
				ID:                 10,
				CampaignID:         1,
				CampaignDailyLimit: 100,
				Width:              300,
				Height:             250,
				ImageURL:           "https://placehold.co/300x250.png",
			},
		},
	}
	cache := newMockAdCache()
	today := time.Now().UTC().Format("2006-01-02")
	// Set 100 impressions already served for campaign today
	cache.dailyCounts[fmt.Sprintf("1:%s", today)] = 100

	svc := NewAdService(crRepo, nil, cache, 3)

	// Request -> should be filtered out because campaign daily limit is reached
	resp, err := svc.ServeAd(context.Background(), 300, 250, "192.168.1.1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response due to daily limit, got %+v", resp)
	}
}
