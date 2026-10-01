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

	// 6. Test /admin dashboard serving
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin", nil)
	wAdmin := httptest.NewRecorder()
	r.ServeHTTP(wAdmin, reqAdmin)

	if wAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /admin, got %d", wAdmin.Code)
	}
	bodyAdmin := wAdmin.Body.String()
	if !strings.Contains(bodyAdmin, "Operations & Admin Console") && !strings.Contains(bodyAdmin, "Mini Ad Server") {
		t.Errorf("expected admin.html content in /admin response")
	}

	// 7. Verify DELETE /campaigns/{id} is registered (not 405 Method Not Allowed)
	reqDelete := httptest.NewRequest(http.MethodDelete, "/campaigns/not-an-id", nil)
	wDelete := httptest.NewRecorder()
	r.ServeHTTP(wDelete, reqDelete)
	if wDelete.Code == http.StatusMethodNotAllowed {
		t.Errorf("expected DELETE /campaigns/{id} to be allowed, got 405 Method Not Allowed")
	}
	if wDelete.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid id, got %d", wDelete.Code)
	}

	// 8. Verify PUT /campaigns/{id} is registered (not 405 Method Not Allowed)
	reqPut := httptest.NewRequest(http.MethodPut, "/campaigns/not-an-id", nil)
	wPut := httptest.NewRecorder()
	r.ServeHTTP(wPut, reqPut)
	if wPut.Code == http.StatusMethodNotAllowed {
		t.Errorf("expected PUT /campaigns/{id} to be allowed, got 405 Method Not Allowed")
	}
	if wPut.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid id, got %d", wPut.Code)
	}

	// 9. Verify DELETE /creatives/{id} is registered (not 405 Method Not Allowed)
	reqDeleteCreative := httptest.NewRequest(http.MethodDelete, "/creatives/not-an-id", nil)
	wDeleteCreative := httptest.NewRecorder()
	r.ServeHTTP(wDeleteCreative, reqDeleteCreative)
	if wDeleteCreative.Code == http.StatusMethodNotAllowed {
		t.Errorf("expected DELETE /creatives/{id} to be allowed, got 405 Method Not Allowed")
	}
	if wDeleteCreative.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid id, got %d", wDeleteCreative.Code)
	}

	// 10. Verify PUT /creatives/{id} is registered (not 405 Method Not Allowed)
	reqPutCreative := httptest.NewRequest(http.MethodPut, "/creatives/not-an-id", nil)
	wPutCreative := httptest.NewRecorder()
	r.ServeHTTP(wPutCreative, reqPutCreative)
	if wPutCreative.Code == http.StatusMethodNotAllowed {
		t.Errorf("expected PUT /creatives/{id} to be allowed, got 405 Method Not Allowed")
	}
	if wPutCreative.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid id, got %d", wPutCreative.Code)
	}

	// 11. Test /config endpoint
	reqConfig := httptest.NewRequest(http.MethodGet, "/config", nil)
	wConfig := httptest.NewRecorder()
	r.ServeHTTP(wConfig, reqConfig)
	if wConfig.Code != http.StatusOK {
		t.Errorf("expected 200 OK for /config, got %d", wConfig.Code)
	}
	bodyConfig := wConfig.Body.String()
	if !strings.Contains(bodyConfig, "app_url") {
		t.Errorf("expected /config response to contain 'app_url', got %s", bodyConfig)
	}
}
