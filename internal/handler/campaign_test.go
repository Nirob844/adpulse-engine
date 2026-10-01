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

func setupTestRouter(t *testing.T) (chi.Router, *repo.CampaignRepo) {
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
		return nil, nil
	}

	_ = db.RunMigrations(ctx, pool, "../../migrations/001_init.sql")

	campaignRepo := repo.NewCampaignRepo(pool)
	campaignService := service.NewCampaignService(campaignRepo)
	campaignHandler := NewCampaignHandler(campaignService)

	r := chi.NewRouter()
	r.Post("/campaigns", campaignHandler.Create)
	r.Patch("/campaigns/{id}", campaignHandler.UpdateStatus)
	r.Get("/campaigns", campaignHandler.List)

	return r, campaignRepo
}

func TestCreateCampaignSuccess(t *testing.T) {
	r, _ := setupTestRouter(t)
	if r == nil {
		return
	}

	payload := map[string]any{
		"name":                   "Test Campaign One",
		"daily_impression_limit": 1000,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/campaigns", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var created model.Campaign
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if created.ID <= 0 {
		t.Errorf("expected positive campaign ID, got %d", created.ID)
	}
	if created.Name != "Test Campaign One" {
		t.Errorf("expected name 'Test Campaign One', got '%s'", created.Name)
	}
	if created.Status != "active" {
		t.Errorf("expected default status 'active', got '%s'", created.Status)
	}
	if created.DailyImpressionLimit != 1000 {
		t.Errorf("expected limit 1000, got %d", created.DailyImpressionLimit)
	}
}

func TestCreateCampaignValidations(t *testing.T) {
	r, _ := setupTestRouter(t)
	if r == nil {
		return
	}

	testCases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "empty name",
			body:       `{"name":"","daily_impression_limit":500}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "whitespace-only name",
			body:       `{"name":"   ","daily_impression_limit":500}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing name",
			body:       `{"daily_impression_limit":500}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing daily_impression_limit",
			body:       `{"name":"No Limit"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "string daily_impression_limit '5'",
			body:       `{"name":"String Limit","daily_impression_limit":"5"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "float daily_impression_limit 1.5",
			body:       `{"name":"Float Limit","daily_impression_limit":1.5}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero daily_impression_limit",
			body:       `{"name":"Zero Limit","daily_impression_limit":0}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative daily_impression_limit",
			body:       `{"name":"Negative Limit","daily_impression_limit":-100}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid status",
			body:       `{"name":"Bad Status","daily_impression_limit":100,"status":"archived"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid with paused status",
			body:       `{"name":"Paused Promo","daily_impression_limit":200,"status":"paused"}`,
			wantStatus: http.StatusCreated,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/campaigns", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("[%s] expected status %d, got %d. Body: %s", tc.name, tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestPatchCampaignStatus(t *testing.T) {
	r, repo := setupTestRouter(t)
	if r == nil {
		return
	}

	// Create a test campaign to patch
	camp, err := repo.Create(context.Background(), "Patch Test Campaign", 1000, "active")
	if err != nil {
		t.Fatalf("failed to create test campaign: %v", err)
	}

	// 1. Valid PATCH to 'paused'
	patchBody := bytes.NewBufferString(`{"status":"paused"}`)
	req := httptest.NewRequest(http.MethodPatch, "/campaigns/"+strconv.FormatInt(camp.ID, 10), patchBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var updated model.Campaign
	_ = json.NewDecoder(w.Body).Decode(&updated)
	if updated.Status != "paused" {
		t.Errorf("expected status 'paused', got '%s'", updated.Status)
	}

	// 2. Valid PATCH to resume ('active')
	patchBody = bytes.NewBufferString(`{"status":"active"}`)
	req = httptest.NewRequest(http.MethodPatch, "/campaigns/"+strconv.FormatInt(camp.ID, 10), patchBody)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	// 3. Invalid status value
	patchBody = bytes.NewBufferString(`{"status":"unknown"}`)
	req = httptest.NewRequest(http.MethodPatch, "/campaigns/"+strconv.FormatInt(camp.ID, 10), patchBody)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid status, got %d", w.Code)
	}

	// 4. Invalid ID parameter (non-integer)
	req = httptest.NewRequest(http.MethodPatch, "/campaigns/abc", bytes.NewBufferString(`{"status":"paused"}`))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-integer id, got %d", w.Code)
	}

	// 5. Unknown campaign ID (404)
	req = httptest.NewRequest(http.MethodPatch, "/campaigns/9999999", bytes.NewBufferString(`{"status":"paused"}`))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown campaign, got %d", w.Code)
	}
}
