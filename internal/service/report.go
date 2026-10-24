package service

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/nirob844/adpulse-engine/internal/model"
)

// Common report business validation errors
var (
	ErrMissingDate       = errors.New("both 'from' and 'to' date parameters are required")
	ErrInvalidDateFormat = errors.New("dates must be formatted as YYYY-MM-DD")
	ErrFromAfterTo       = errors.New("'from' date cannot be after 'to' date")
	ErrInvalidGroupBy    = errors.New("group_by must be either 'campaign' or 'day'")
)

// ReportEventRepo defines event querying interface for aggregated reporting.
type ReportEventRepo interface {
	GetCampaignReport(ctx context.Context, fromTime, toTime time.Time) ([]*model.CampaignReportRow, error)
	GetDayReport(ctx context.Context, fromTime, toTime time.Time) ([]*model.DayReportRow, error)
}

// ReportService orchestrates report generation, date validation, and CTR metric calculation.
type ReportService struct {
	repo ReportEventRepo
}

// NewReportService creates a new ReportService.
func NewReportService(repo ReportEventRepo) *ReportService {
	return &ReportService{repo: repo}
}

// GenerateReport validates dates, executes the chosen aggregation, and computes CTR per group.
func (s *ReportService) GenerateReport(ctx context.Context, fromStr, toStr, groupBy string) (any, error) {
	fromClean := strings.TrimSpace(fromStr)
	toClean := strings.TrimSpace(toStr)

	if fromClean == "" || toClean == "" {
		return nil, ErrMissingDate
	}

	fromDate, err := time.Parse("2006-01-02", fromClean)
	if err != nil {
		return nil, ErrInvalidDateFormat
	}

	toDate, err := time.Parse("2006-01-02", toClean)
	if err != nil {
		return nil, ErrInvalidDateFormat
	}

	if fromDate.After(toDate) {
		return nil, ErrFromAfterTo
	}

	// Normalize group_by (defaults to "campaign")
	groupClean := strings.ToLower(strings.TrimSpace(groupBy))
	if groupClean == "" {
		groupClean = "campaign"
	}
	if groupClean != "campaign" && groupClean != "day" {
		return nil, ErrInvalidGroupBy
	}

	// Calculate inclusive time window: from 00:00:00 UTC to toDate+1 day 00:00:00 UTC
	fromTime := time.Date(fromDate.Year(), fromDate.Month(), fromDate.Day(), 0, 0, 0, 0, time.UTC)
	toTime := time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)

	if groupClean == "campaign" {
		rows, err := s.repo.GetCampaignReport(ctx, fromTime, toTime)
		if err != nil {
			return nil, err
		}
		if rows == nil {
			rows = []*model.CampaignReportRow{}
		}

		// Calculate CTR for each campaign group
		for _, row := range rows {
			row.CTR = calculateCTR(row.Clicks, row.Impressions)
		}
		return rows, nil
	}

	// Group by day
	rows, err := s.repo.GetDayReport(ctx, fromTime, toTime)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []*model.DayReportRow{}
	}

	// Calculate CTR for each day group
	for _, row := range rows {
		row.CTR = calculateCTR(row.Clicks, row.Impressions)
	}
	return rows, nil
}

// calculateCTR computes CTR as (clicks / impressions) * 100, rounded to 2 decimal places.
// Returns 0.0 when impressions are 0.
func calculateCTR(clicks, impressions int64) float64 {
	if impressions <= 0 {
		return 0.0
	}
	rawCTR := (float64(clicks) / float64(impressions)) * 100.0
	return math.Round(rawCTR*100) / 100.0
}
