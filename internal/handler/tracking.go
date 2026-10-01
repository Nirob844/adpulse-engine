package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/nirob844/mini-ad-server/internal/service"
)

// Standard 43-byte 1x1 transparent GIF binary sequence
var transparent1x1GIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00,
	0x80, 0x00, 0x00, 0xff, 0xff, 0xff, 0x00, 0x00, 0x00, 0x21,
	0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00,
	0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44,
	0x01, 0x00, 0x3b,
}

// TrackingHandler handles impression pixel and click redirection requests.
type TrackingHandler struct {
	service *service.TrackingService
}

// NewTrackingHandler creates a new TrackingHandler.
func NewTrackingHandler(s *service.TrackingService) *TrackingHandler {
	return &TrackingHandler{service: s}
}

// TrackImpression handles GET /track/impression?c=<creative_id>
// Records an impression event and returns a 1x1 transparent GIF (200 OK).
func (h *TrackingHandler) TrackImpression(w http.ResponseWriter, r *http.Request) {
	cParam := strings.TrimSpace(r.URL.Query().Get("c"))
	if cParam == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "creative id query parameter 'c' is required")
		return
	}

	creativeID, err := strconv.ParseInt(cParam, 10, 64)
	if err != nil || creativeID <= 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "creative id 'c' must be a positive integer")
		return
	}

	ip := getClientIP(r)
	userAgent := r.UserAgent()

	if err := h.service.TrackImpression(r.Context(), creativeID, ip, userAgent); err != nil {
		if errors.Is(err, service.ErrCreativeNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "creative not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to record impression: "+err.Error())
		return
	}

	// Disable caching to ensure every load fires an impression
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Content-Length", strconv.Itoa(len(transparent1x1GIF)))
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(transparent1x1GIF)
}

// TrackClick handles GET /track/click?c=<creative_id>
// Records a click event and responds with a 302 redirect to the creative's click_url.
func (h *TrackingHandler) TrackClick(w http.ResponseWriter, r *http.Request) {
	cParam := strings.TrimSpace(r.URL.Query().Get("c"))
	if cParam == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "creative id query parameter 'c' is required")
		return
	}

	creativeID, err := strconv.ParseInt(cParam, 10, 64)
	if err != nil || creativeID <= 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "creative id 'c' must be a positive integer")
		return
	}

	ip := getClientIP(r)
	userAgent := r.UserAgent()

	landingURL, err := h.service.TrackClick(r.Context(), creativeID, ip, userAgent)
	if err != nil {
		if errors.Is(err, service.ErrCreativeNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "creative not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to record click: "+err.Error())
		return
	}

	http.Redirect(w, r, landingURL, http.StatusFound)
}
