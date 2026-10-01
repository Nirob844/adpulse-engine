package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nirob844/mini-ad-server/internal/config"
	"github.com/nirob844/mini-ad-server/internal/db"
	"github.com/nirob844/mini-ad-server/internal/model"
	"github.com/nirob844/mini-ad-server/internal/repo"
	"github.com/nirob844/mini-ad-server/internal/service"
)

func setupReportTestRouter(t *testing.T) (chi.Router, *repo.AdEventRepo, *repo.CreativeRepo, *repo.CampaignRepo) {
	_ = os.Unsetenv("DATABASE_URL")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("skipping live database test: %v", err)
		return nil, nil, nil, nil
	}

	_ = db.RunMigrations(ctx, pool, "../../migrations/001_init.sql")

	campaignRepo := repo.NewCampaignRepo(pool)
	creativeRepo := repo.NewCreativeRepo(pool)
	eventRepo := repo.NewAdEventRepo(pool)

	reportService := service.NewReportService(eventRepo)
	reportHandler := NewReportHandler(reportService)

	r := chi.NewRouter()
	r.Get("/report", reportHandler.GetReport)

	return r, eventRepo, creativeRepo, campaignRepo
}

func TestGetReportValidation(t *testing.T) {
	r, _, _, _ := setupReportTestRouter(t)
	if r == nil {
		return
	}

	testCases := []struct {
		name       string
		url        string
		wantStatus int
	}{
		{
			name:       "missing from and to",
			url:        "/report",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing to",
			url:        "/report?from=2026-10-01",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid date format",
			url:        "/report?from=01-10-2026&to=2026-10-02",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "from after to",
			url:        "/report?from=2026-10-05&to=2026-10-01",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown group_by",
			url:        "/report?from=2026-10-01&to=2026-10-02&group_by=month",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("[%s] expected status %d, got %d. Body: %s", tc.name, tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestGetReportSuccess(t *testing.T) {
	r, eventRepo, creativeRepo, campRepo := setupReportTestRouter(t)
	if r == nil {
		return
	}

	// 1. Seed campaign, creative, and events
	camp, err := campRepo.Create(context.Background(), "Report Test Campaign", 1000, "active")
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	cr, err := creativeRepo.Create(context.Background(), camp.ID, 300, 250, "https://placehold.co/300x250.png", "https://example.com/target")
	if err != nil {
		t.Fatalf("failed to create creative: %v", err)
	}

	// Insert 2 impressions and 1 click
	_, _ = eventRepo.RecordEvent(context.Background(), camp.ID, cr.ID, "impression", "127.0.0.1", "Agent")
	_, _ = eventRepo.RecordEvent(context.Background(), camp.ID, cr.ID, "impression", "127.0.0.2", "Agent")
	_, _ = eventRepo.RecordEvent(context.Background(), camp.ID, cr.ID, "click", "127.0.0.1", "Agent")

	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := now.AddDate(0, 0, 1).Format("2006-01-02")

	// 2. Test group_by=campaign (default)
	req := httptest.NewRequest(http.MethodGet, "/report?from="+yesterday+"&to="+tomorrow, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for campaign report, got %d: %s", w.Code, w.Body.String())
	}

	var campRows []*model.CampaignReportRow
	if err := json.NewDecoder(w.Body).Decode(&campRows); err != nil {
		t.Fatalf("failed to decode campaign report: %v", err)
	}

	if len(campRows) == 0 {
		t.Fatalf("expected at least 1 campaign row in report")
	}

	found := false
	for _, row := range campRows {
		if row.CampaignID == camp.ID {
			found = true
			if row.Impressions != 2 {
				t.Errorf("expected 2 impressions, got %d", row.Impressions)
			}
			if row.Clicks != 1 {
				t.Errorf("expected 1 click, got %d", row.Clicks)
			}
			if row.CTR != 50.0 {
				t.Errorf("expected CTR 50.0%% (1/2*100), got %f", row.CTR)
			}
		}
	}
	if !found {
		t.Errorf("expected to find campaign %d in report", camp.ID)
	}

	// 3. Test group_by=day
	reqDay := httptest.NewRequest(http.MethodGet, "/report?from="+today+"&to="+today+"&group_by=day", nil)
	wDay := httptest.NewRecorder()
	r.ServeHTTP(wDay, reqDay)

	if wDay.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for day report, got %d: %s", wDay.Code, wDay.Body.String())
	}

	var dayRows []*model.DayReportRow
	if err := json.NewDecoder(wDay.Body).Decode(&dayRows); err != nil {
		t.Fatalf("failed to decode day report: %v", err)
	}

	if len(dayRows) == 0 {
		t.Fatalf("expected at least 1 day row in report")
	}
}
