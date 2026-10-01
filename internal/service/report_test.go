package service

import (
	"context"
	"testing"
	"time"

	"github.com/nirob844/mini-ad-server/internal/model"
)

type mockReportRepo struct {
	campaignRows []*model.CampaignReportRow
	dayRows      []*model.DayReportRow
}

func (m *mockReportRepo) GetCampaignReport(ctx context.Context, fromTime, toTime time.Time) ([]*model.CampaignReportRow, error) {
	return m.campaignRows, nil
}

func (m *mockReportRepo) GetDayReport(ctx context.Context, fromTime, toTime time.Time) ([]*model.DayReportRow, error) {
	return m.dayRows, nil
}

func TestReportService_Validations(t *testing.T) {
	repo := &mockReportRepo{}
	svc := NewReportService(repo)

	testCases := []struct {
		name      string
		from      string
		to        string
		groupBy   string
		wantErr   error
	}{
		{
			name:    "missing from",
			from:    "",
			to:      "2026-10-01",
			groupBy: "campaign",
			wantErr: ErrMissingDate,
		},
		{
			name:    "missing to",
			from:    "2026-10-01",
			to:      "",
			groupBy: "campaign",
			wantErr: ErrMissingDate,
		},
		{
			name:    "invalid from date format",
			from:    "01-10-2026",
			to:      "2026-10-02",
			groupBy: "campaign",
			wantErr: ErrInvalidDateFormat,
		},
		{
			name:    "invalid to date format",
			from:    "2026-10-01",
			to:      "invalid-date",
			groupBy: "campaign",
			wantErr: ErrInvalidDateFormat,
		},
		{
			name:    "from date after to date",
			from:    "2026-10-05",
			to:      "2026-10-01",
			groupBy: "campaign",
			wantErr: ErrFromAfterTo,
		},
		{
			name:    "invalid group_by parameter",
			from:    "2026-10-01",
			to:      "2026-10-02",
			groupBy: "month",
			wantErr: ErrInvalidGroupBy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.GenerateReport(context.Background(), tc.from, tc.to, tc.groupBy)
			if err != tc.wantErr {
				t.Errorf("[%s] expected error %v, got %v", tc.name, tc.wantErr, err)
			}
		})
	}
}

func TestReportService_CTRCalculation(t *testing.T) {
	repo := &mockReportRepo{
		campaignRows: []*model.CampaignReportRow{
			{CampaignID: 1, CampaignName: "Camp 1", Impressions: 100, Clicks: 5},
			{CampaignID: 2, CampaignName: "Camp 2", Impressions: 0, Clicks: 0},
			{CampaignID: 3, CampaignName: "Camp 3", Impressions: 300, Clicks: 7}, // 7/300*100 = 2.3333... -> 2.33
		},
		dayRows: []*model.DayReportRow{
			{Date: "2026-10-01", Impressions: 50, Clicks: 2}, // 2/50*100 = 4.0
		},
	}
	svc := NewReportService(repo)

	// 1. Group by campaign
	campRes, err := svc.GenerateReport(context.Background(), "2026-10-01", "2026-10-02", "campaign")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	campRows, ok := campRes.([]*model.CampaignReportRow)
	if !ok || len(campRows) != 3 {
		t.Fatalf("expected 3 campaign rows, got %+v", campRes)
	}

	if campRows[0].CTR != 5.0 {
		t.Errorf("expected CTR 5.0, got %f", campRows[0].CTR)
	}
	if campRows[1].CTR != 0.0 {
		t.Errorf("expected CTR 0.0 for 0 impressions, got %f", campRows[1].CTR)
	}
	if campRows[2].CTR != 2.33 {
		t.Errorf("expected CTR 2.33, got %f", campRows[2].CTR)
	}

	// 2. Group by day
	dayRes, err := svc.GenerateReport(context.Background(), "2026-10-01", "2026-10-02", "day")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dayRows, ok := dayRes.([]*model.DayReportRow)
	if !ok || len(dayRows) != 1 {
		t.Fatalf("expected 1 day row, got %+v", dayRes)
	}
	if dayRows[0].CTR != 4.0 {
		t.Errorf("expected CTR 4.0, got %f", dayRows[0].CTR)
	}
}
