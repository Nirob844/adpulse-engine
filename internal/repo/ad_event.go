package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nirob844/mini-ad-server/internal/model"
)

// AdEventRepo handles database operations for ad impressions and clicks.
type AdEventRepo struct {
	pool *pgxpool.Pool
}

// NewAdEventRepo creates a new AdEventRepo.
func NewAdEventRepo(pool *pgxpool.Pool) *AdEventRepo {
	return &AdEventRepo{pool: pool}
}

// RecordEvent inserts an ad event (impression or click) into the database.
func (r *AdEventRepo) RecordEvent(ctx context.Context, campaignID, creativeID int64, eventType, ip, userAgent string) (*model.AdEvent, error) {
	query := `
		INSERT INTO ad_events (campaign_id, creative_id, type, ip, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, campaign_id, creative_id, type, ip, user_agent, created_at
	`
	event := &model.AdEvent{}
	err := r.pool.QueryRow(ctx, query, campaignID, creativeID, eventType, ip, userAgent).Scan(
		&event.ID,
		&event.CampaignID,
		&event.CreativeID,
		&event.Type,
		&event.IP,
		&event.UserAgent,
		&event.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record ad event: %w", err)
	}
	return event, nil
}

// CountDailyImpressions returns the number of impressions for a campaign on a given UTC date.
func (r *AdEventRepo) CountDailyImpressions(ctx context.Context, campaignID int64, date time.Time) (int64, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := `
		SELECT COUNT(*)
		FROM ad_events
		WHERE campaign_id = $1 
		  AND type = 'impression' 
		  AND created_at >= $2 
		  AND created_at < $3
	`
	var count int64
	err := r.pool.QueryRow(ctx, query, campaignID, startOfDay, endOfDay).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count daily impressions: %w", err)
	}
	return count, nil
}

// CountCreativeIPImpressions returns the number of impressions served to an IP for a creative on a given UTC date.
func (r *AdEventRepo) CountCreativeIPImpressions(ctx context.Context, creativeID int64, ip string, date time.Time) (int64, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := `
		SELECT COUNT(*)
		FROM ad_events
		WHERE creative_id = $1 
		  AND ip = $2 
		  AND type = 'impression' 
		  AND created_at >= $3 
		  AND created_at < $4
	`
	var count int64
	err := r.pool.QueryRow(ctx, query, creativeID, ip, startOfDay, endOfDay).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count creative ip impressions: %w", err)
	}
	return count, nil
}

// GetCampaignReport aggregates impressions and clicks per campaign for the given time range.
func (r *AdEventRepo) GetCampaignReport(ctx context.Context, fromTime, toTime time.Time) ([]*model.CampaignReportRow, error) {
	query := `
		SELECT 
			c.id AS campaign_id,
			c.name AS campaign_name,
			COUNT(CASE WHEN e.type = 'impression' THEN 1 END) AS impressions,
			COUNT(CASE WHEN e.type = 'click' THEN 1 END) AS clicks
		FROM ad_events e
		JOIN campaigns c ON e.campaign_id = c.id
		WHERE e.created_at >= $1 AND e.created_at < $2
		GROUP BY c.id, c.name
		ORDER BY c.id ASC
	`
	rows, err := r.pool.Query(ctx, query, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query campaign report: %w", err)
	}
	defer rows.Close()

	var report []*model.CampaignReportRow
	for rows.Next() {
		row := &model.CampaignReportRow{}
		if err := rows.Scan(
			&row.CampaignID,
			&row.CampaignName,
			&row.Impressions,
			&row.Clicks,
		); err != nil {
			return nil, fmt.Errorf("failed to scan campaign report row: %w", err)
		}
		report = append(report, row)
	}
	return report, rows.Err()
}

// GetDayReport aggregates impressions and clicks per day for the given time range.
func (r *AdEventRepo) GetDayReport(ctx context.Context, fromTime, toTime time.Time) ([]*model.DayReportRow, error) {
	query := `
		SELECT 
			TO_CHAR(e.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS report_date,
			COUNT(CASE WHEN e.type = 'impression' THEN 1 END) AS impressions,
			COUNT(CASE WHEN e.type = 'click' THEN 1 END) AS clicks
		FROM ad_events e
		WHERE e.created_at >= $1 AND e.created_at < $2
		GROUP BY report_date
		ORDER BY report_date ASC
	`
	rows, err := r.pool.Query(ctx, query, fromTime, toTime)
	if err != nil {
		return nil, fmt.Errorf("failed to query day report: %w", err)
	}
	defer rows.Close()

	var report []*model.DayReportRow
	for rows.Next() {
		row := &model.DayReportRow{}
		if err := rows.Scan(
			&row.Date,
			&row.Impressions,
			&row.Clicks,
		); err != nil {
			return nil, fmt.Errorf("failed to scan day report row: %w", err)
		}
		report = append(report, row)
	}
	return report, rows.Err()
}
