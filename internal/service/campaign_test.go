package service

import (
	"context"
	"testing"
	"time"

	"github.com/nirob844/mini-ad-server/internal/model"
)

type mockCampaignRepo struct {
	campaigns map[int64]*model.Campaign
	nextID    int64
}

func newMockCampaignRepo() *mockCampaignRepo {
	return &mockCampaignRepo{
		campaigns: make(map[int64]*model.Campaign),
		nextID:    1,
	}
}

func (m *mockCampaignRepo) Create(ctx context.Context, name string, dailyLimit int, status string) (*model.Campaign, error) {
	camp := &model.Campaign{
		ID:                   m.nextID,
		Name:                 name,
		Status:               status,
		DailyImpressionLimit: dailyLimit,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	m.campaigns[m.nextID] = camp
	m.nextID++
	return camp, nil
}

func (m *mockCampaignRepo) UpdateStatus(ctx context.Context, id int64, status string) (*model.Campaign, error) {
	camp, ok := m.campaigns[id]
	if !ok {
		return nil, ErrCampaignNotFound
	}
	camp.Status = status
	camp.UpdatedAt = time.Now()
	return camp, nil
}

func (m *mockCampaignRepo) GetByID(ctx context.Context, id int64) (*model.Campaign, error) {
	camp, ok := m.campaigns[id]
	if !ok {
		return nil, ErrCampaignNotFound
	}
	return camp, nil
}

func (m *mockCampaignRepo) ListAll(ctx context.Context) ([]*model.Campaign, error) {
	list := make([]*model.Campaign, 0, len(m.campaigns))
	for _, c := range m.campaigns {
		list = append(list, c)
	}
	return list, nil
}

func TestCampaignService_CreateCampaign(t *testing.T) {
	repo := newMockCampaignRepo()
	svc := NewCampaignService(repo)

	// Valid creation
	camp, err := svc.CreateCampaign(context.Background(), "Brand X", 2000, "active")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if camp.ID != 1 || camp.Name != "Brand X" || camp.Status != "active" {
		t.Errorf("unexpected campaign: %+v", camp)
	}

	// Empty name
	_, err = svc.CreateCampaign(context.Background(), "  ", 2000, "active")
	if err != ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}

	// Non-positive limit
	_, err = svc.CreateCampaign(context.Background(), "Brand Y", 0, "active")
	if err != ErrInvalidLimit {
		t.Errorf("expected ErrInvalidLimit, got %v", err)
	}

	// Invalid status
	_, err = svc.CreateCampaign(context.Background(), "Brand Z", 100, "unknown")
	if err != ErrInvalidStatus {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestCampaignService_UpdateStatus(t *testing.T) {
	repo := newMockCampaignRepo()
	svc := NewCampaignService(repo)

	camp, _ := svc.CreateCampaign(context.Background(), "Brand A", 500, "active")

	// Update to paused
	updated, err := svc.UpdateStatus(context.Background(), camp.ID, "paused")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Status != "paused" {
		t.Errorf("expected status 'paused', got '%s'", updated.Status)
	}

	// Update non-existent campaign
	_, err = svc.UpdateStatus(context.Background(), 999, "active")
	if err != ErrCampaignNotFound {
		t.Errorf("expected ErrCampaignNotFound, got %v", err)
	}

	// Update with invalid status
	_, err = svc.UpdateStatus(context.Background(), camp.ID, "invalid_status")
	if err != ErrInvalidStatus {
		t.Errorf("expected ErrInvalidStatus, got %v", err)
	}
}
