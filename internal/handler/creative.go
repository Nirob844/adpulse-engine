package handler

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nirob844/mini-ad-server/internal/model"
	"github.com/nirob844/mini-ad-server/internal/service"
)

// CreativeHandler handles creative HTTP requests.
type CreativeHandler struct {
	service *service.CreativeService
}

// NewCreativeHandler creates a new CreativeHandler.
func NewCreativeHandler(s *service.CreativeService) *CreativeHandler {
	return &CreativeHandler{service: s}
}

type createCreativePayload struct {
	CampaignID any `json:"campaign_id"`
	Width      any `json:"width"`
	Height     any `json:"height"`
	ImageURL   any `json:"image_url"`
	ClickURL   any `json:"click_url"`
}

// Create handles POST /creatives
func (h *CreativeHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "request body is required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var payload createCreativePayload
	if err := decoder.Decode(&payload); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request payload: "+err.Error())
		return
	}

	// 1. Validate campaign_id (required positive integer)
	if payload.CampaignID == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "campaign_id is required")
		return
	}
	campNum, ok := payload.CampaignID.(json.Number)
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "campaign_id must be a number, string values are not allowed")
		return
	}
	campRaw := campNum.String()
	if strings.Contains(campRaw, ".") || strings.ContainsAny(campRaw, "eE") {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "campaign_id must be an integer, floats are not allowed")
		return
	}
	campID, err := campNum.Int64()
	if err != nil || campID <= 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "campaign_id must be a positive integer")
		return
	}

	// 2. Validate width (required positive integer)
	if payload.Width == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "width is required")
		return
	}
	wNum, ok := payload.Width.(json.Number)
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "width must be a number, string values are not allowed")
		return
	}
	wRaw := wNum.String()
	if strings.Contains(wRaw, ".") || strings.ContainsAny(wRaw, "eE") {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "width must be an integer, floats are not allowed")
		return
	}
	widthVal, err := wNum.Int64()
	if err != nil || widthVal <= 0 || widthVal > math.MaxInt32 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "width must be a positive integer")
		return
	}

	// 3. Validate height (required positive integer)
	if payload.Height == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "height is required")
		return
	}
	hNum, ok := payload.Height.(json.Number)
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "height must be a number, string values are not allowed")
		return
	}
	hRaw := hNum.String()
	if strings.Contains(hRaw, ".") || strings.ContainsAny(hRaw, "eE") {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "height must be an integer, floats are not allowed")
		return
	}
	heightVal, err := hNum.Int64()
	if err != nil || heightVal <= 0 || heightVal > math.MaxInt32 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "height must be a positive integer")
		return
	}

	// 4. Validate image_url (required string)
	if payload.ImageURL == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "image_url is required")
		return
	}
	imageURLStr, ok := payload.ImageURL.(string)
	if !ok || strings.TrimSpace(imageURLStr) == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "image_url must be a non-empty string")
		return
	}

	// 5. Validate click_url (required string)
	if payload.ClickURL == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "click_url is required")
		return
	}
	clickURLStr, ok := payload.ClickURL.(string)
	if !ok || strings.TrimSpace(clickURLStr) == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "click_url must be a non-empty string")
		return
	}

	// 6. Delegate to Service
	req := model.CreateCreativeRequest{
		CampaignID: campID,
		Width:      int(widthVal),
		Height:     int(heightVal),
		ImageURL:   strings.TrimSpace(imageURLStr),
		ClickURL:   strings.TrimSpace(clickURLStr),
	}

	cr, err := h.service.CreateCreative(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrCampaignNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "campaign not found")
			return
		}
		if errors.Is(err, service.ErrInvalidImageURL) || errors.Is(err, service.ErrInvalidClickURL) ||
			errors.Is(err, service.ErrInvalidWidth) || errors.Is(err, service.ErrInvalidHeight) ||
			errors.Is(err, service.ErrInvalidCampaignID) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to create creative: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, cr)
}

// GetByID handles GET /creatives/:id
func (h *CreativeHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid creative id, must be a positive integer")
		return
	}

	cr, err := h.service.GetCreative(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrCreativeNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "creative not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to get creative: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, cr)
}

// List handles GET /creatives (supports optional query param ?campaign_id=...)
func (h *CreativeHandler) List(w http.ResponseWriter, r *http.Request) {
	var campaignID *int64
	campQuery := strings.TrimSpace(r.URL.Query().Get("campaign_id"))
	if campQuery != "" {
		id, err := strconv.ParseInt(campQuery, 10, 64)
		if err != nil || id <= 0 {
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid campaign_id query parameter, must be a positive integer")
			return
		}
		campaignID = &id
	}

	creatives, err := h.service.ListCreatives(r.Context(), campaignID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to list creatives: "+err.Error())
		return
	}
	if creatives == nil {
		creatives = []*model.Creative{}
	}

	WriteJSON(w, http.StatusOK, creatives)
}
