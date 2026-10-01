package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nirob844/mini-ad-server/internal/model"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

// CampaignRepo handles database operations for campaigns.
type CampaignRepo struct {
	pool *pgxpool.Pool
}

// NewCampaignRepo creates a new CampaignRepo.
func NewCampaignRepo(pool *pgxpool.Pool) *CampaignRepo {
	return &CampaignRepo{pool: pool}
}

// Create inserts a new campaign into the database.
func (r *CampaignRepo) Create(ctx context.Context, name string, dailyLimit int, status string) (*model.Campaign, error) {
	query := `
		INSERT INTO campaigns (name, status, daily_impression_limit)
		VALUES ($1, $2, $3)
		RETURNING id, name, status, daily_impression_limit, created_at, updated_at
	`
	camp := &model.Campaign{}
	err := r.pool.QueryRow(ctx, query, name, status, dailyLimit).Scan(
		&camp.ID,
		&camp.Name,
		&camp.Status,
		&camp.DailyImpressionLimit,
		&camp.CreatedAt,
		&camp.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create campaign: %w", err)
	}
	return camp, nil
}

// UpdateStatus updates the status and updated_at timestamp of a campaign.
func (r *CampaignRepo) UpdateStatus(ctx context.Context, id int64, status string) (*model.Campaign, error) {
	query := `
		UPDATE campaigns
		SET status = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING id, name, status, daily_impression_limit, created_at, updated_at
	`
	camp := &model.Campaign{}
	err := r.pool.QueryRow(ctx, query, status, id).Scan(
		&camp.ID,
		&camp.Name,
		&camp.Status,
		&camp.DailyImpressionLimit,
		&camp.CreatedAt,
		&camp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to update campaign status: %w", err)
	}
	return camp, nil
}

// Update updates all editable fields of a campaign (name, daily_impression_limit, status).
func (r *CampaignRepo) Update(ctx context.Context, id int64, name string, dailyLimit int, status string) (*model.Campaign, error) {
	query := `
		UPDATE campaigns
		SET name = $1, daily_impression_limit = $2, status = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING id, name, status, daily_impression_limit, created_at, updated_at
	`
	camp := &model.Campaign{}
	err := r.pool.QueryRow(ctx, query, name, dailyLimit, status, id).Scan(
		&camp.ID,
		&camp.Name,
		&camp.Status,
		&camp.DailyImpressionLimit,
		&camp.CreatedAt,
		&camp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to update campaign: %w", err)
	}
	return camp, nil
}

// Delete removes a campaign by ID. Cascades to associated creatives and ad_events via DB foreign keys.
func (r *CampaignRepo) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM campaigns WHERE id = $1`
	cmdTag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete campaign: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID retrieves a campaign by its primary key ID.
func (r *CampaignRepo) GetByID(ctx context.Context, id int64) (*model.Campaign, error) {
	query := `
		SELECT id, name, status, daily_impression_limit, created_at, updated_at
		FROM campaigns
		WHERE id = $1
	`
	camp := &model.Campaign{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&camp.ID,
		&camp.Name,
		&camp.Status,
		&camp.DailyImpressionLimit,
		&camp.CreatedAt,
		&camp.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}
	return camp, nil
}

// ListAll returns all campaigns ordered by ID.
func (r *CampaignRepo) ListAll(ctx context.Context) ([]*model.Campaign, error) {
	query := `
		SELECT id, name, status, daily_impression_limit, created_at, updated_at
		FROM campaigns
		ORDER BY id ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list campaigns: %w", err)
	}
	defer rows.Close()

	var campaigns []*model.Campaign
	for rows.Next() {
		camp := &model.Campaign{}
		if err := rows.Scan(
			&camp.ID,
			&camp.Name,
			&camp.Status,
			&camp.DailyImpressionLimit,
			&camp.CreatedAt,
			&camp.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan campaign: %w", err)
		}
		campaigns = append(campaigns, camp)
	}
	return campaigns, rows.Err()
}
