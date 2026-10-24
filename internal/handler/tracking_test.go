package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nirob844/adpulse-engine/internal/config"
	"github.com/nirob844/adpulse-engine/internal/db"
	"github.com/nirob844/adpulse-engine/internal/repo"
	"github.com/nirob844/adpulse-engine/internal/service"
)

func setupTrackingTestRouter(t *testing.T) (chi.Router, *repo.CreativeRepo, *repo.CampaignRepo) {
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
		return nil, nil, nil
	}

	_ = db.RunMigrations(ctx, pool, "../../migrations/001_init.sql")

	campaignRepo := repo.NewCampaignRepo(pool)
	creativeRepo := repo.NewCreativeRepo(pool)
	eventRepo := repo.NewAdEventRepo(pool)

	trackingService := service.NewTrackingService(creativeRepo, eventRepo, nil)
	trackingHandler := NewTrackingHandler(trackingService)

	r := chi.NewRouter()
	r.Get("/track/impression", trackingHandler.TrackImpression)
	r.Get("/track/click", trackingHandler.TrackClick)

	return r, creativeRepo, campaignRepo
}

func TestTrackImpression(t *testing.T) {
	r, creativeRepo, campRepo := setupTrackingTestRouter(t)
	if r == nil {
		return
	}

	// 1. Setup creative
	camp, err := campRepo.Create(context.Background(), "Tracking Imp Campaign", 5000, "active")
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}
	cr, err := creativeRepo.Create(context.Background(), camp.ID, 300, 250, "https://placehold.co/300x250.png", "https://example.com/target")
	if err != nil {
		t.Fatalf("failed to create creative: %v", err)
	}

	// 2. Success: Valid c parameter
	req := httptest.NewRequest(http.MethodGet, "/track/impression?c="+strconv.FormatInt(cr.ID, 10), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "image/gif" {
		t.Errorf("expected Content-Type 'image/gif', got '%s'", w.Header().Get("Content-Type"))
	}
	if w.Body.Len() != 43 {
		t.Errorf("expected 43 byte GIF, got %d bytes", w.Body.Len())
	}
	if !bytes.Equal(w.Body.Bytes(), transparent1x1GIF) {
		t.Errorf("body does not match 1x1 transparent GIF")
	}
	if w.Header().Get("Cache-Control") != "no-store, no-cache, must-revalidate, max-age=0" {
		t.Errorf("unexpected Cache-Control: %s", w.Header().Get("Cache-Control"))
	}

	// 3. Unknown creative (404)
	req404 := httptest.NewRequest(http.MethodGet, "/track/impression?c=999999", nil)
	w404 := httptest.NewRecorder()
	r.ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown creative, got %d", w404.Code)
	}

	// 4. Missing c param (400)
	req400 := httptest.NewRequest(http.MethodGet, "/track/impression", nil)
	w400 := httptest.NewRecorder()
	r.ServeHTTP(w400, req400)
	if w400.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing c param, got %d", w400.Code)
	}

	// 5. Non-integer c param (400)
	reqBadID := httptest.NewRequest(http.MethodGet, "/track/impression?c=abc", nil)
	wBadID := httptest.NewRecorder()
	r.ServeHTTP(wBadID, reqBadID)
	if wBadID.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-integer c, got %d", wBadID.Code)
	}
}

func TestTrackClick(t *testing.T) {
	r, creativeRepo, campRepo := setupTrackingTestRouter(t)
	if r == nil {
		return
	}

	// 1. Setup creative
	camp, err := campRepo.Create(context.Background(), "Tracking Click Campaign", 5000, "active")
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}
	cr, err := creativeRepo.Create(context.Background(), camp.ID, 728, 90, "https://placehold.co/728x90.png", "https://example.com/special-offer")
	if err != nil {
		t.Fatalf("failed to create creative: %v", err)
	}

	// 2. Success: Valid c parameter -> 302 Found redirect
	req := httptest.NewRequest(http.MethodGet, "/track/click?c="+strconv.FormatInt(cr.ID, 10), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302 Found, got %d", w.Code)
	}
	if w.Header().Get("Location") != "https://example.com/special-offer" {
		t.Errorf("expected Location 'https://example.com/special-offer', got '%s'", w.Header().Get("Location"))
	}

	// 3. Unknown creative (404)
	req404 := httptest.NewRequest(http.MethodGet, "/track/click?c=999999", nil)
	w404 := httptest.NewRecorder()
	r.ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown creative, got %d", w404.Code)
	}

	// 4. Missing c param (400)
	req400 := httptest.NewRequest(http.MethodGet, "/track/click", nil)
	w400 := httptest.NewRecorder()
	r.ServeHTTP(w400, req400)
	if w400.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing c param, got %d", w400.Code)
	}
}
