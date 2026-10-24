package service

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/nirob844/adpulse-engine/internal/model"
	"github.com/nirob844/adpulse-engine/internal/repo"
)

// Common creative business logic errors
var (
	ErrInvalidCreativeID = errors.New("invalid creative id")
	ErrInvalidCampaignID = errors.New("campaign_id must be a positive integer")
	ErrInvalidWidth      = errors.New("width must be a positive integer")
	ErrInvalidHeight     = errors.New("height must be a positive integer")
	ErrInvalidImageURL   = errors.New("image_url must be a valid http or https URL")
	ErrInvalidClickURL   = errors.New("click_url must be a valid http or https URL")
	ErrCreativeNotFound  = errors.New("creative not found")
)

// CreativeRepository defines the database interface required by CreativeService.
type CreativeRepository interface {
	Create(ctx context.Context, campaignID int64, width, height int, imageURL, clickURL string) (*model.Creative, error)
	GetByID(ctx context.Context, id int64) (*model.Creative, error)
	ListAll(ctx context.Context) ([]*model.Creative, error)
	ListByCampaignID(ctx context.Context, campaignID int64) ([]*model.Creative, error)
	GetActiveCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, error)
	Update(ctx context.Context, id, campaignID int64, width, height int, imageURL, clickURL string) (*model.Creative, error)
	Delete(ctx context.Context, id int64) error
}

// CreativeService coordinates creative business logic.
type CreativeService struct {
	creativeRepo CreativeRepository
	campaignRepo CampaignRepository
}

// NewCreativeService creates a new CreativeService.
func NewCreativeService(cr CreativeRepository, ca CampaignRepository) *CreativeService {
	return &CreativeService{
		creativeRepo: cr,
		campaignRepo: ca,
	}
}

// CreateCreative validates business rules and creates a new creative under a campaign.
func (s *CreativeService) CreateCreative(ctx context.Context, req model.CreateCreativeRequest) (*model.Creative, error) {
	if req.CampaignID <= 0 {
		return nil, ErrInvalidCampaignID
	}
	if req.Width <= 0 {
		return nil, ErrInvalidWidth
	}
	if req.Height <= 0 {
		return nil, ErrInvalidHeight
	}

	imageURL := strings.TrimSpace(req.ImageURL)
	if err := validateURL(imageURL); err != nil {
		return nil, ErrInvalidImageURL
	}

	clickURL := strings.TrimSpace(req.ClickURL)
	if err := validateURL(clickURL); err != nil {
		return nil, ErrInvalidClickURL
	}

	// Verify campaign exists
	if s.campaignRepo != nil {
		_, err := s.campaignRepo.GetByID(ctx, req.CampaignID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				return nil, ErrCampaignNotFound
			}
			return nil, err
		}
	}

	cr, err := s.creativeRepo.Create(ctx, req.CampaignID, req.Width, req.Height, imageURL, clickURL)
	if err != nil {
		if errors.Is(err, repo.ErrForeignKeyViolation) {
			return nil, ErrCampaignNotFound
		}
		return nil, err
	}
	return cr, nil
}

// GetCreative retrieves a creative by ID.
func (s *CreativeService) GetCreative(ctx context.Context, id int64) (*model.Creative, error) {
	if id <= 0 {
		return nil, ErrInvalidCreativeID
	}

	cr, err := s.creativeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrCreativeNotFound
		}
		return nil, err
	}
	return cr, nil
}

// ListCreatives lists all creatives, optionally filtered by campaign ID.
func (s *CreativeService) ListCreatives(ctx context.Context, campaignID *int64) ([]*model.Creative, error) {
	if campaignID != nil && *campaignID > 0 {
		return s.creativeRepo.ListByCampaignID(ctx, *campaignID)
	}
	return s.creativeRepo.ListAll(ctx)
}

// GetActiveCandidates retrieves candidate creatives for ad serving.
func (s *CreativeService) GetActiveCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, error) {
	if width <= 0 || height <= 0 {
		return nil, errors.New("dimensions must be positive")
	}
	return s.creativeRepo.GetActiveCandidates(ctx, width, height)
}

// UpdateCreative validates business rules and updates an existing creative.
func (s *CreativeService) UpdateCreative(ctx context.Context, id int64, req model.CreateCreativeRequest) (*model.Creative, error) {
	if id <= 0 {
		return nil, ErrInvalidCreativeID
	}
	if req.CampaignID <= 0 {
		return nil, ErrInvalidCampaignID
	}
	if req.Width <= 0 {
		return nil, ErrInvalidWidth
	}
	if req.Height <= 0 {
		return nil, ErrInvalidHeight
	}

	imageURL := strings.TrimSpace(req.ImageURL)
	if err := validateURL(imageURL); err != nil {
		return nil, ErrInvalidImageURL
	}

	clickURL := strings.TrimSpace(req.ClickURL)
	if err := validateURL(clickURL); err != nil {
		return nil, ErrInvalidClickURL
	}

	// Verify campaign exists
	if s.campaignRepo != nil {
		_, err := s.campaignRepo.GetByID(ctx, req.CampaignID)
		if err != nil {
			if errors.Is(err, repo.ErrNotFound) {
				return nil, ErrCampaignNotFound
			}
			return nil, err
		}
	}

	cr, err := s.creativeRepo.Update(ctx, id, req.CampaignID, req.Width, req.Height, imageURL, clickURL)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ErrCreativeNotFound
		}
		if errors.Is(err, repo.ErrForeignKeyViolation) {
			return nil, ErrCampaignNotFound
		}
		return nil, err
	}
	return cr, nil
}

// DeleteCreative removes a creative by ID.
func (s *CreativeService) DeleteCreative(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidCreativeID
	}

	err := s.creativeRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrCreativeNotFound
		}
		return err
	}
	return nil
}

// validateURL checks if a string is a valid absolute HTTP or HTTPS URL.
func validateURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("url is empty")
	}
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("scheme must be http or https")
	}
	if u.Host == "" {
		return errors.New("host is required")
	}
	return nil
}
