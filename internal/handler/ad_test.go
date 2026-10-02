package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nirob844/mini-ad-server/internal/config"
	"github.com/nirob844/mini-ad-server/internal/db"
	"github.com/nirob844/mini-ad-server/internal/model"
	"github.com/nirob844/mini-ad-server/internal/repo"
	"github.com/nirob844/mini-ad-server/internal/service"
)

func setupAdTestRouter(t *testing.T) (chi.Router, *repo.CreativeRepo, *repo.CampaignRepo) {
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

	adService := service.NewAdService(creativeRepo, eventRepo, nil, 3)
	adHandler := NewAdHandler(adService)

	r := chi.NewRouter()
	r.Get("/ad", adHandler.ServeAd)

	return r, creativeRepo, campaignRepo
}

func TestServeAdValidation(t *testing.T) {
	r, _, _ := setupAdTestRouter(t)
	if r == nil {
		return
	}

	testCases := []struct {
		name       string
		url        string
		wantStatus int
	}{
		{
			name:       "missing size parameter",
			url:        "/ad",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty size parameter",
			url:        "/ad?size=",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid format no height",
			url:        "/ad?size=300",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid non-integer width",
			url:        "/ad?size=abcx250",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid non-integer height",
			url:        "/ad?size=300xxyz",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative width",
			url:        "/ad?size=-300x250",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero height",
			url:        "/ad?size=300x0",
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

func TestServeAdSuccessAndNoContent(t *testing.T) {
	r, creativeRepo, campRepo := setupAdTestRouter(t)
	if r == nil {
		return
	}

	// 1. Ensure an active campaign with a 300x250 creative exists
	camp, err := campRepo.Create(context.Background(), "Ad Test Campaign", 10000, "active")
	if err != nil {
		t.Fatalf("failed to create campaign: %v", err)
	}

	_, err = creativeRepo.Create(context.Background(), camp.ID, 300, 250, "https://placehold.co/300x250.png", "https://example.com/target")
	if err != nil {
		t.Fatalf("failed to create creative: %v", err)
	}

	// 2. Request matching 300x250 ad
	req := httptest.NewRequest(http.MethodGet, "/ad?size=300x250", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp model.AdResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode ad response: %v", err)
	}

	if resp.CreativeID <= 0 {
		t.Errorf("expected positive creative_id, got %d", resp.CreativeID)
	}
	if !strings.Contains(resp.HTML, "<img") || !strings.Contains(resp.HTML, `width="300"`) {
		t.Errorf("expected <img> tag in HTML, got: %s", resp.HTML)
	}
	if !strings.HasPrefix(resp.ImpressionURL, "/track/impression?c=") {
		t.Errorf("expected impression tracking URL prefix, got: %s", resp.ImpressionURL)
	}
	if !strings.HasPrefix(resp.ClickURL, "/track/click?c=") {
		t.Errorf("expected click tracking URL prefix, got: %s", resp.ClickURL)
	}

	// 3. Request case-insensitive dimension "300X250"
	reqCase := httptest.NewRequest(http.MethodGet, "/ad?size=300X250", nil)
	wCase := httptest.NewRecorder()
	r.ServeHTTP(wCase, reqCase)
	if wCase.Code != http.StatusOK {
		t.Errorf("expected 200 OK for uppercase 'X', got %d", wCase.Code)
	}

	// 4. Request size with NO matching creatives -> Expect 204 No Content
	reqNoMatch := httptest.NewRequest(http.MethodGet, "/ad?size=999x888", nil)
	wNoMatch := httptest.NewRecorder()
	r.ServeHTTP(wNoMatch, reqNoMatch)

	if wNoMatch.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for size 999x888, got %d: %s", wNoMatch.Code, wNoMatch.Body.String())
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name:       "Simulated IP query parameter takes top priority",
			url:        "/ad?size=300x250&ip=192.168.1.99",
			headers:    map[string]string{"X-Real-IP": "10.0.0.1", "X-Forwarded-For": "172.16.0.1"},
			remoteAddr: "127.0.0.1:8000",
			expectedIP: "192.168.1.99",
		},
		{
			name:       "X-Real-IP header priority over RemoteAddr",
			url:        "/ad?size=300x250",
			headers:    map[string]string{"X-Real-IP": "203.0.113.195"},
			remoteAddr: "127.0.0.1:8000",
			expectedIP: "203.0.113.195",
		},
		{
			name:       "X-Forwarded-For first IP in list",
			url:        "/ad?size=300x250",
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.42, 10.0.0.1, 127.0.0.1"},
			remoteAddr: "127.0.0.1:8000",
			expectedIP: "198.51.100.42",
		},
		{
			name:       "IPv4 RemoteAddr with port",
			url:        "/ad?size=300x250",
			remoteAddr: "192.168.1.50:49281",
			expectedIP: "192.168.1.50",
		},
		{
			name:       "IPv6 RemoteAddr with port",
			url:        "/ad?size=300x250",
			remoteAddr: "[2001:db8::1]:54321",
			expectedIP: "2001:db8::1",
		},
		{
			name:       "IPv6 localhost with port",
			url:        "/ad?size=300x250",
			remoteAddr: "[::1]:50000",
			expectedIP: "::1",
		},
		{
			name:       "Plain IP without port fallback",
			url:        "/ad?size=300x250",
			remoteAddr: "10.10.10.10",
			expectedIP: "10.10.10.10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tc.remoteAddr

			got := getClientIP(req)
			if got != tc.expectedIP {
				t.Errorf("expected IP %q, got %q", tc.expectedIP, got)
			}
		})
	}
}
