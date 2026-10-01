package model

import (
	"time"
)

// CampaignStatus constants.
const (
	CampaignStatusActive = "active"
	CampaignStatusPaused = "paused"
)

// EventType constants.
const (
	EventTypeImpression = "impression"
	EventTypeClick      = "click"
)

// Campaign represents an advertising campaign.
type Campaign struct {
	ID                   int64     `json:"id" db:"id"`
	Name                 string    `json:"name" db:"name"`
	Status               string    `json:"status" db:"status"`
	DailyImpressionLimit int       `json:"daily_impression_limit" db:"daily_impression_limit"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
}

// Creative represents an ad creative banner under a campaign.
type Creative struct {
	ID         int64     `json:"id" db:"id"`
	CampaignID int64     `json:"campaign_id" db:"campaign_id"`
	Width      int       `json:"width" db:"width"`
	Height     int       `json:"height" db:"height"`
	ImageURL   string    `json:"image_url" db:"image_url"`
	ClickURL   string    `json:"click_url" db:"click_url"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// AdEvent records an impression or click event.
type AdEvent struct {
	ID         int64     `json:"id" db:"id"`
	CampaignID int64     `json:"campaign_id" db:"campaign_id"`
	CreativeID int64     `json:"creative_id" db:"creative_id"`
	Type       string    `json:"type" db:"type"`
	IP         string    `json:"ip" db:"ip"`
	UserAgent  string    `json:"user_agent" db:"user_agent"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// AdResponse is the JSON payload returned by GET /ad?size=WxH
type AdResponse struct {
	CreativeID    int64  `json:"creative_id"`
	HTML          string `json:"html"`
	ImpressionURL string `json:"impression_url"`
	ClickURL      string `json:"click_url"`
}

// CampaignReportRow represents aggregated metrics grouped by campaign.
type CampaignReportRow struct {
	CampaignID   int64   `json:"campaign_id"`
	CampaignName string  `json:"campaign_name"`
	Impressions  int64   `json:"impressions"`
	Clicks       int64   `json:"clicks"`
	CTR          float64 `json:"ctr"`
}

// DayReportRow represents aggregated metrics grouped by day.
type DayReportRow struct {
	Date        string  `json:"date"`
	Impressions int64   `json:"impressions"`
	Clicks      int64   `json:"clicks"`
	CTR         float64 `json:"ctr"`
}

// CreateCampaignRequest defines input for POST /campaigns
type CreateCampaignRequest struct {
	Name                 string  `json:"name"`
	DailyImpressionLimit int     `json:"daily_impression_limit"`
	Status               *string `json:"status,omitempty"`
}

// UpdateCampaignRequest defines input for PATCH /campaigns/:id
type UpdateCampaignRequest struct {
	Name                 *string `json:"name,omitempty"`
	Status               *string `json:"status,omitempty"`
	DailyImpressionLimit *int    `json:"daily_impression_limit,omitempty"`
}

// CreateCreativeRequest defines input for creating a new creative.
type CreateCreativeRequest struct {
	CampaignID int64  `json:"campaign_id"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	ImageURL   string `json:"image_url"`
	ClickURL   string `json:"click_url"`
}
