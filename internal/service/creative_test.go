package service

import (
	"context"
	"testing"
	"time"

	"github.com/nirob844/mini-ad-server/internal/model"
)

type mockCreativeRepo struct {
	creatives map[int64]*model.Creative
	nextID    int64
}

func newMockCreativeRepo() *mockCreativeRepo {
	return &mockCreativeRepo{
		creatives: make(map[int64]*model.Creative),
		nextID:    1,
	}
}

func (m *mockCreativeRepo) Create(ctx context.Context, campaignID int64, width, height int, imageURL, clickURL string) (*model.Creative, error) {
	cr := &model.Creative{
		ID:         m.nextID,
		CampaignID: campaignID,
		Width:      width,
		Height:     height,
		ImageURL:   imageURL,
		ClickURL:   clickURL,
		CreatedAt:  time.Now(),
	}
	m.creatives[m.nextID] = cr
	m.nextID++
	return cr, nil
}

func (m *mockCreativeRepo) GetByID(ctx context.Context, id int64) (*model.Creative, error) {
	cr, ok := m.creatives[id]
	if !ok {
		return nil, ErrCreativeNotFound
	}
	return cr, nil
}

func (m *mockCreativeRepo) ListAll(ctx context.Context) ([]*model.Creative, error) {
	list := make([]*model.Creative, 0, len(m.creatives))
	for _, c := range m.creatives {
		list = append(list, c)
	}
	return list, nil
}

func (m *mockCreativeRepo) ListByCampaignID(ctx context.Context, campaignID int64) ([]*model.Creative, error) {
	var list []*model.Creative
	for _, c := range m.creatives {
		if c.CampaignID == campaignID {
			list = append(list, c)
		}
	}
	return list, nil
}

func (m *mockCreativeRepo) GetActiveCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, error) {
	var list []*model.CreativeCandidate
	for _, c := range m.creatives {
		if c.Width == width && c.Height == height {
			list = append(list, &model.CreativeCandidate{
				ID:                 c.ID,
				CampaignID:         c.CampaignID,
				CampaignName:       "Test Campaign",
				CampaignDailyLimit: 1000,
				Width:              c.Width,
				Height:             c.Height,
				ImageURL:           c.ImageURL,
				ClickURL:           c.ClickURL,
			})
		}
	}
	return list, nil
}

func TestCreativeService_CreateCreative(t *testing.T) {
	campRepo := newMockCampaignRepo()
	camp, _ := campRepo.Create(context.Background(), "Brand X", 1000, "active")

	crRepo := newMockCreativeRepo()
	svc := NewCreativeService(crRepo, campRepo)

	// Valid creative creation
	req := model.CreateCreativeRequest{
		CampaignID: camp.ID,
		Width:      300,
		Height:     250,
		ImageURL:   "https://placehold.co/300x250.png",
		ClickURL:   "https://example.com/laptop",
	}
	cr, err := svc.CreateCreative(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if cr.ID != 1 || cr.CampaignID != camp.ID || cr.Width != 300 || cr.Height != 250 {
		t.Errorf("unexpected creative: %+v", cr)
	}

	// Invalid campaign ID
	badReq := req
	badReq.CampaignID = 0
	if _, err := svc.CreateCreative(context.Background(), badReq); err != ErrInvalidCampaignID {
		t.Errorf("expected ErrInvalidCampaignID, got %v", err)
	}

	// Campaign not found
	badReq.CampaignID = 999
	if _, err := svc.CreateCreative(context.Background(), badReq); err != ErrCampaignNotFound {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}

	// Invalid dimensions
	badReq = req
	badReq.Width = -10
	if _, err := svc.CreateCreative(context.Background(), badReq); err != ErrInvalidWidth {
		t.Errorf("expected ErrInvalidWidth, got %v", err)
	}

	badReq = req
	badReq.Height = 0
	if _, err := svc.CreateCreative(context.Background(), badReq); err != ErrInvalidHeight {
		t.Errorf("expected ErrInvalidHeight, got %v", err)
	}

	// Invalid URLs
	badReq = req
	badReq.ImageURL = "not-a-valid-url"
	if _, err := svc.CreateCreative(context.Background(), badReq); err != ErrInvalidImageURL {
		t.Errorf("expected ErrInvalidImageURL, got %v", err)
	}

	badReq = req
	badReq.ClickURL = "ftp://invalid-scheme.com"
	if _, err := svc.CreateCreative(context.Background(), badReq); err != ErrInvalidClickURL {
		t.Errorf("expected ErrInvalidClickURL, got %v", err)
	}
}

func TestCreativeService_GetAndList(t *testing.T) {
	campRepo := newMockCampaignRepo()
	camp, _ := campRepo.Create(context.Background(), "Brand X", 1000, "active")

	crRepo := newMockCreativeRepo()
	svc := NewCreativeService(crRepo, campRepo)

	req := model.CreateCreativeRequest{
		CampaignID: camp.ID,
		Width:      728,
		Height:     90,
		ImageURL:   "https://placehold.co/728x90.png",
		ClickURL:   "https://example.com/banner",
	}
	created, err := svc.CreateCreative(context.Background(), req)
	if err != nil {
		t.Fatalf("failed to create creative: %v", err)
	}

	// Get by ID
	found, err := svc.GetCreative(context.Background(), created.ID)
	if err != nil || found.ID != created.ID {
		t.Fatalf("failed to get creative by id: %v", err)
	}

	// Get non-existent
	if _, err := svc.GetCreative(context.Background(), 999); err != ErrCreativeNotFound {
		t.Errorf("expected ErrCreativeNotFound, got %v", err)
	}

	// List all
	list, err := svc.ListCreatives(context.Background(), nil)
	if err != nil || len(list) != 1 {
		t.Errorf("expected 1 creative, got %d", len(list))
	}

	// List by campaign
	campList, err := svc.ListCreatives(context.Background(), &camp.ID)
	if err != nil || len(campList) != 1 {
		t.Errorf("expected 1 creative for campaign, got %d", len(campList))
	}

	// Candidates
	cands, err := svc.GetActiveCandidates(context.Background(), 728, 90)
	if err != nil || len(cands) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(cands))
	}
}
