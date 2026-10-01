package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nirob844/mini-ad-server/internal/config"
	"github.com/nirob844/mini-ad-server/internal/db"
	"github.com/nirob844/mini-ad-server/internal/model"
	"github.com/nirob844/mini-ad-server/internal/repo"
	"github.com/nirob844/mini-ad-server/internal/service"
)

func setupCreativeTestRouter(t *testing.T) (chi.Router, *repo.CreativeRepo, *repo.CampaignRepo) {
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
	creativeService := service.NewCreativeService(creativeRepo, campaignRepo)

	creativeHandler := NewCreativeHandler(creativeService)

	r := chi.NewRouter()
	r.Post("/creatives", creativeHandler.Create)
	r.Get("/creatives", creativeHandler.List)
	r.Get("/creatives/{id}", creativeHandler.GetByID)

	return r, creativeRepo, campaignRepo
}

func TestCreateCreativeSuccess(t *testing.T) {
	r, _, campRepo := setupCreativeTestRouter(t)
	if r == nil {
		return
	}

	// Create test campaign
	camp, err := campRepo.Create(context.Background(), "Creative Test Campaign", 2500, "active")
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	payload := map[string]any{
		"campaign_id": camp.ID,
		"width":       300,
		"height":      250,
		"image_url":   "https://placehold.co/300x250.png",
		"click_url":   "https://example.com/product",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/creatives", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var created model.Creative
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if created.ID <= 0 {
		t.Errorf("expected positive creative ID, got %d", created.ID)
	}
	if created.CampaignID != camp.ID {
		t.Errorf("expected campaign ID %d, got %d", camp.ID, created.CampaignID)
	}
	if created.Width != 300 || created.Height != 250 {
		t.Errorf("expected 300x250, got %dx%d", created.Width, created.Height)
	}
	if created.ImageURL != "https://placehold.co/300x250.png" {
		t.Errorf("unexpected image_url: %s", created.ImageURL)
	}
	if created.ClickURL != "https://example.com/product" {
		t.Errorf("unexpected click_url: %s", created.ClickURL)
	}
}

func TestCreateCreativeValidations(t *testing.T) {
	r, _, campRepo := setupCreativeTestRouter(t)
	if r == nil {
		return
	}

	camp, err := campRepo.Create(context.Background(), "Creative Val Campaign", 1000, "active")
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	campIDStr := strconv.FormatInt(camp.ID, 10)

	testCases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "missing campaign_id",
			body:       `{"width":300,"height":250,"image_url":"https://placehold.co/300x250.png","click_url":"https://example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "string campaign_id",
			body:       `{"campaign_id":"` + campIDStr + `","width":300,"height":250,"image_url":"https://placehold.co/300x250.png","click_url":"https://example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-existent campaign_id",
			body:       `{"campaign_id":999999,"width":300,"height":250,"image_url":"https://placehold.co/300x250.png","click_url":"https://example.com"}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "zero width",
			body:       `{"campaign_id":` + campIDStr + `,"width":0,"height":250,"image_url":"https://placehold.co/300x250.png","click_url":"https://example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative height",
			body:       `{"campaign_id":` + campIDStr + `,"width":300,"height":-250,"image_url":"https://placehold.co/300x250.png","click_url":"https://example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "float width",
			body:       `{"campaign_id":` + campIDStr + `,"width":300.5,"height":250,"image_url":"https://placehold.co/300x250.png","click_url":"https://example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid image_url protocol",
			body:       `{"campaign_id":` + campIDStr + `,"width":300,"height":250,"image_url":"javascript:alert(1)","click_url":"https://example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty click_url",
			body:       `{"campaign_id":` + campIDStr + `,"width":300,"height":250,"image_url":"https://placehold.co/300x250.png","click_url":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown field disallowed",
			body:       `{"campaign_id":` + campIDStr + `,"width":300,"height":250,"image_url":"https://placehold.co/300x250.png","click_url":"https://example.com","extra_field":"danger"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/creatives", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("[%s] expected status %d, got %d. Body: %s", tc.name, tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestGetAndListCreatives(t *testing.T) {
	r, _, campRepo := setupCreativeTestRouter(t)
	if r == nil {
		return
	}

	camp, err := campRepo.Create(context.Background(), "Creative List Test", 500, "active")
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	// 1. Create a creative
	payload := map[string]any{
		"campaign_id": camp.ID,
		"width":       728,
		"height":      90,
		"image_url":   "https://placehold.co/728x90.png",
		"click_url":   "https://example.com/banner",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/creatives", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create creative: %s", w.Body.String())
	}

	var created model.Creative
	_ = json.NewDecoder(w.Body).Decode(&created)

	// 2. GET /creatives/:id
	req = httptest.NewRequest(http.MethodGet, "/creatives/"+strconv.FormatInt(created.ID, 10), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Code)
	}

	// 3. GET /creatives/999999 (not found)
	req = httptest.NewRequest(http.MethodGet, "/creatives/999999", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown creative, got %d", w.Code)
	}

	// 4. GET /creatives?campaign_id=<id>
	req = httptest.NewRequest(http.MethodGet, "/creatives?campaign_id="+strconv.FormatInt(camp.ID, 10), nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK for list by campaign, got %d", w.Code)
	}

	var list []*model.Creative
	_ = json.NewDecoder(w.Body).Decode(&list)
	if len(list) == 0 {
		t.Errorf("expected at least 1 creative in list")
	}
}
