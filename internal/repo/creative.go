package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nirob844/mini-ad-server/internal/model"
)

// ErrForeignKeyViolation is returned when a foreign key constraint is violated.
var ErrForeignKeyViolation = errors.New("foreign key violation")

// CreativeRepo handles database operations for ad creatives.
type CreativeRepo struct {
	pool *pgxpool.Pool
}

// NewCreativeRepo creates a new CreativeRepo.
func NewCreativeRepo(pool *pgxpool.Pool) *CreativeRepo {
	return &CreativeRepo{pool: pool}
}

// Create inserts a new creative into the database.
func (r *CreativeRepo) Create(ctx context.Context, campaignID int64, width, height int, imageURL, clickURL string) (*model.Creative, error) {
	query := `
		INSERT INTO creatives (campaign_id, width, height, image_url, click_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, campaign_id, width, height, image_url, click_url, created_at
	`
	cr := &model.Creative{}
	err := r.pool.QueryRow(ctx, query, campaignID, width, height, imageURL, clickURL).Scan(
		&cr.ID,
		&cr.CampaignID,
		&cr.Width,
		&cr.Height,
		&cr.ImageURL,
		&cr.ClickURL,
		&cr.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, ErrForeignKeyViolation
		}
		return nil, fmt.Errorf("failed to create creative: %w", err)
	}
	return cr, nil
}

// GetByID retrieves a creative by its ID.
func (r *CreativeRepo) GetByID(ctx context.Context, id int64) (*model.Creative, error) {
	query := `
		SELECT id, campaign_id, width, height, image_url, click_url, created_at
		FROM creatives
		WHERE id = $1
	`
	cr := &model.Creative{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&cr.ID,
		&cr.CampaignID,
		&cr.Width,
		&cr.Height,
		&cr.ImageURL,
		&cr.ClickURL,
		&cr.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get creative: %w", err)
	}
	return cr, nil
}

// ListAll returns all creatives ordered by ID.
func (r *CreativeRepo) ListAll(ctx context.Context) ([]*model.Creative, error) {
	query := `
		SELECT id, campaign_id, width, height, image_url, click_url, created_at
		FROM creatives
		ORDER BY id ASC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list creatives: %w", err)
	}
	defer rows.Close()

	var creatives []*model.Creative
	for rows.Next() {
		cr := &model.Creative{}
		if err := rows.Scan(
			&cr.ID,
			&cr.CampaignID,
			&cr.Width,
			&cr.Height,
			&cr.ImageURL,
			&cr.ClickURL,
			&cr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan creative: %w", err)
		}
		creatives = append(creatives, cr)
	}
	return creatives, rows.Err()
}

// ListByCampaignID returns all creatives for a specific campaign.
func (r *CreativeRepo) ListByCampaignID(ctx context.Context, campaignID int64) ([]*model.Creative, error) {
	query := `
		SELECT id, campaign_id, width, height, image_url, click_url, created_at
		FROM creatives
		WHERE campaign_id = $1
		ORDER BY id ASC
	`
	rows, err := r.pool.Query(ctx, query, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to list creatives by campaign: %w", err)
	}
	defer rows.Close()

	var creatives []*model.Creative
	for rows.Next() {
		cr := &model.Creative{}
		if err := rows.Scan(
			&cr.ID,
			&cr.CampaignID,
			&cr.Width,
			&cr.Height,
			&cr.ImageURL,
			&cr.ClickURL,
			&cr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan creative: %w", err)
		}
		creatives = append(creatives, cr)
	}
	return creatives, rows.Err()
}

// GetActiveCandidates retrieves creatives belonging to active campaigns matching requested dimensions.
func (r *CreativeRepo) GetActiveCandidates(ctx context.Context, width, height int) ([]*model.CreativeCandidate, error) {
	query := `
		SELECT 
			cr.id,
			cr.campaign_id,
			ca.name AS campaign_name,
			ca.daily_impression_limit AS campaign_daily_limit,
			cr.width,
			cr.height,
			cr.image_url,
			cr.click_url
		FROM creatives cr
		JOIN campaigns ca ON cr.campaign_id = ca.id
		WHERE ca.status = 'active' AND cr.width = $1 AND cr.height = $2
		ORDER BY cr.id ASC
	`
	rows, err := r.pool.Query(ctx, query, width, height)
	if err != nil {
		return nil, fmt.Errorf("failed to query active creative candidates: %w", err)
	}
	defer rows.Close()

	var candidates []*model.CreativeCandidate
	for rows.Next() {
		c := &model.CreativeCandidate{}
		if err := rows.Scan(
			&c.ID,
			&c.CampaignID,
			&c.CampaignName,
			&c.CampaignDailyLimit,
			&c.Width,
			&c.Height,
			&c.ImageURL,
			&c.ClickURL,
		); err != nil {
			return nil, fmt.Errorf("failed to scan creative candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}
