package service

import (
	"context"
	"errors"
	"strings"

	"github.com/nirob844/mini-ad-server/internal/model"
	"github.com/nirob844/mini-ad-server/internal/repo"
)

// Common business logic errors
var (
	ErrInvalidName      = errors.New("name is required and cannot be empty")
	ErrInvalidLimit     = errors.New("daily_impression_limit must be a positive integer")
	ErrInvalidStatus    = errors.New("status must be either 'active' or 'paused'")
	ErrCampaignNotFound = errors.New("campaign not found")
)

// CampaignRepository defines the database interface required by CampaignService.
type CampaignRepository interface {
	Create(ctx context.Context, name string, dailyLimit int, status string) (*model.Campaign, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*model.Campaign, error)
	Update(ctx context.Context, id int64, name string, dailyLimit int, status string) (*model.Campaign, error)
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*model.Campaign, error)
	ListAll(ctx context.Context) ([]*model.Campaign, error)
}

// CampaignService orchestrates campaign business logic and coordinates repository/cache.
type CampaignService struct {
	repo CampaignRepository
}

// NewCampaignService creates a new CampaignService.
func NewCampaignService(r CampaignRepository) *CampaignService {
	return &CampaignService{repo: r}
}

// CreateCampaign validates business rules and creates a new campaign.
func (s *CampaignService) CreateCampaign(ctx context.Context, name string, dailyLimit int, status string) (*model.Campaign, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrInvalidName
	}
	if dailyLimit <= 0 {
		return nil, ErrInvalidLimit
	}

	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus == "" {
		trimmedStatus = model.CampaignStatusActive
	}
	if trimmedStatus != model.CampaignStatusActive && trimmedStatus != model.CampaignStatusPaused {
		return nil, ErrInvalidStatus
	}

	return s.repo.Create(ctx, trimmedName, dailyLimit, trimmedStatus)
}

// UpdateStatus validates and updates campaign status.
func (s *CampaignService) UpdateStatus(ctx context.Context, id int64, status string) (*model.Campaign, error) {
	if id <= 0 {
		return nil, errors.New("invalid campaign id")
	}

	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus != model.CampaignStatusActive && trimmedStatus != model.CampaignStatusPaused {
		return nil, ErrInvalidStatus
	}

	camp, err := s.repo.UpdateStatus(ctx, id, trimmedStatus)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrCampaignNotFound
		}
		return nil, err
	}

	return camp, nil
}

// ListCampaigns returns all campaigns.
func (s *CampaignService) ListCampaigns(ctx context.Context) ([]*model.Campaign, error) {
	return s.repo.ListAll(ctx)
}

// GetCampaign retrieves a single campaign by ID.
func (s *CampaignService) GetCampaign(ctx context.Context, id int64) (*model.Campaign, error) {
	if id <= 0 {
		return nil, errors.New("invalid campaign id")
	}

	camp, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrCampaignNotFound
		}
		return nil, err
	}

	return camp, nil
}

// UpdateCampaign validates business rules and updates all campaign fields.
func (s *CampaignService) UpdateCampaign(ctx context.Context, id int64, name string, dailyLimit int, status string) (*model.Campaign, error) {
	if id <= 0 {
		return nil, errors.New("invalid campaign id")
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, ErrInvalidName
	}
	if dailyLimit <= 0 {
		return nil, ErrInvalidLimit
	}
	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus != model.CampaignStatusActive && trimmedStatus != model.CampaignStatusPaused {
		return nil, ErrInvalidStatus
	}

	camp, err := s.repo.Update(ctx, id, trimmedName, dailyLimit, trimmedStatus)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrCampaignNotFound
		}
		return nil, err
	}
	return camp, nil
}

// DeleteCampaign validates and deletes a campaign by ID.
func (s *CampaignService) DeleteCampaign(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid campaign id")
	}
	err := s.repo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrCampaignNotFound
		}
		return err
	}
	return nil
}
