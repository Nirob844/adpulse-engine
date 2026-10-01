package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterHealthAndCORS(t *testing.T) {
	r := NewRouter(RouterConfig{
		Pool:   nil, // pool nil handles gracefully
		Config: nil,
		WebDir: "../../web",
	})

	// 1. Test Health endpoint
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected health check 200 OK, got %d", w.Code)
	}

	// 2. Test CORS headers
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected CORS Allow-Origin '*', got '%s'", w.Header().Get("Access-Control-Allow-Origin"))
	}

	// 3. Test CORS OPTIONS preflight
	reqOpts := httptest.NewRequest(http.MethodOptions, "/campaigns", nil)
	wOpts := httptest.NewRecorder()
	r.ServeHTTP(wOpts, reqOpts)

	if wOpts.Code != http.StatusNoContent {
		t.Errorf("expected 204 No Content for OPTIONS, got %d", wOpts.Code)
	}
}
