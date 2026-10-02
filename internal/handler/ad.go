package handler

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/nirob844/mini-ad-server/internal/service"
)

// AdHandler handles the ad serving HTTP requests.
type AdHandler struct {
	service *service.AdService
}

// NewAdHandler creates a new AdHandler.
func NewAdHandler(s *service.AdService) *AdHandler {
	return &AdHandler{service: s}
}

// ServeAd handles GET /ad?size=WxH
func (h *AdHandler) ServeAd(w http.ResponseWriter, r *http.Request) {
	sizeQuery := strings.TrimSpace(r.URL.Query().Get("size"))
	if sizeQuery == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "size query parameter is required (e.g. ?size=300x250)")
		return
	}

	width, height, err := parseDimensions(sizeQuery)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid size parameter: "+err.Error())
		return
	}

	// Extract client IP for frequency capping
	clientIP := getClientIP(r)

	adResp, err := h.service.ServeAd(r.Context(), width, height, clientIP)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to serve ad: "+err.Error())
		return
	}

	// If no eligible creative matches size or caps, respond with 204 No Content
	if adResp == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	WriteJSON(w, http.StatusOK, adResp)
}

// parseDimensions parses dimension strings like "300x250" or "728X90" into width and height integers.
func parseDimensions(sizeStr string) (int, int, error) {
	parts := strings.FieldsFunc(sizeStr, func(r rune) bool {
		return r == 'x' || r == 'X'
	})

	if len(parts) != 2 {
		return 0, 0, errors.New("must be in format {width}x{height} (e.g. 300x250)")
	}

	width, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || width <= 0 {
		return 0, 0, errors.New("width must be a positive integer")
	}

	height, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || height <= 0 {
		return 0, 0, errors.New("height must be a positive integer")
	}

	return width, height, nil
}

// getClientIP extracts the real client IP address from query param (?ip=), standard proxy headers, or RemoteAddr.
func getClientIP(r *http.Request) string {
	if simIP := strings.TrimSpace(r.URL.Query().Get("ip")); simIP != "" {
		return simIP
	}
	if xRealIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); xRealIP != "" {
		return xRealIP
	}
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		parts := strings.Split(xForwardedFor, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}
	// Fall back to RemoteAddr (reliably strip port using net.SplitHostPort for both IPv4 and IPv6)
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
