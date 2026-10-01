package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

	// 4. Test publisher.html serving
	reqPub := httptest.NewRequest(http.MethodGet, "/publisher.html", nil)
	wPub := httptest.NewRecorder()
	r.ServeHTTP(wPub, reqPub)

	if wPub.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /publisher.html, got %d", wPub.Code)
	}
	bodyPub := wPub.Body.String()
	if !strings.Contains(bodyPub, "728x90") || !strings.Contains(bodyPub, "300x250") {
		t.Errorf("expected publisher.html to contain 728x90 and 300x250 ad slots")
	}

	// 5. Test adtag.js serving
	reqTag := httptest.NewRequest(http.MethodGet, "/adtag.js", nil)
	wTag := httptest.NewRecorder()
	r.ServeHTTP(wTag, reqTag)

	if wTag.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /adtag.js, got %d", wTag.Code)
	}
	bodyTag := wTag.Body.String()
	if !strings.Contains(bodyTag, "MiniAdTag") {
		t.Errorf("expected adtag.js to define MiniAdTag")
	}
}
