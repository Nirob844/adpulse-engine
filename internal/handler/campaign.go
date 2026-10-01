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

// CampaignHandler handles campaign management HTTP requests via CampaignService.
type CampaignHandler struct {
	service *service.CampaignService
}

// NewCampaignHandler creates a new CampaignHandler.
func NewCampaignHandler(s *service.CampaignService) *CampaignHandler {
	return &CampaignHandler{service: s}
}

type createCampaignPayload struct {
	Name                 any `json:"name"`
	DailyImpressionLimit any `json:"daily_impression_limit"`
	Status               any `json:"status,omitempty"`
}

type updateCampaignPayload struct {
	Status any `json:"status"`
}

// Create handles POST /campaigns
func (h *CampaignHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "request body is required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var payload createCampaignPayload
	if err := decoder.Decode(&payload); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request payload: "+err.Error())
		return
	}

	// 1. Validate name (required, non-empty string)
	if payload.Name == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "name is required")
		return
	}
	nameStr, ok := payload.Name.(string)
	if !ok || strings.TrimSpace(nameStr) == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "name must be a non-empty string")
		return
	}
	name := strings.TrimSpace(nameStr)

	// 2. Validate daily_impression_limit (required positive integer; reject "5", 1.5, <= 0)
	if payload.DailyImpressionLimit == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "daily_impression_limit is required")
		return
	}
	num, ok := payload.DailyImpressionLimit.(json.Number)
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "daily_impression_limit must be a number, string values are not allowed")
		return
	}
	rawLimit := num.String()
	if strings.Contains(rawLimit, ".") || strings.ContainsAny(rawLimit, "eE") {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "daily_impression_limit must be an integer, floats are not allowed")
		return
	}
	limitVal, err := num.Int64()
	if err != nil || limitVal <= 0 || limitVal > math.MaxInt32 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "daily_impression_limit must be a positive integer")
		return
	}

	// 3. Status handling (optional, default: "active", allowed: "active" | "paused")
	status := model.CampaignStatusActive
	if payload.Status != nil {
		statusStr, ok := payload.Status.(string)
		if !ok {
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "status must be a string")
			return
		}
		trimmedStatus := strings.TrimSpace(statusStr)
		if trimmedStatus != model.CampaignStatusActive && trimmedStatus != model.CampaignStatusPaused {
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "status must be either 'active' or 'paused'")
			return
		}
		status = trimmedStatus
	}

	// 4. Delegate to CampaignService
	camp, err := h.service.CreateCampaign(r.Context(), name, int(limitVal), status)
	if err != nil {
		if errors.Is(err, service.ErrInvalidName) || errors.Is(err, service.ErrInvalidLimit) || errors.Is(err, service.ErrInvalidStatus) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to create campaign: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, camp)
}

// UpdateStatus handles PATCH /campaigns/:id
func (h *CampaignHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid campaign id, must be a positive integer")
		return
	}

	if r.Body == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "request body is required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var payload updateCampaignPayload
	if err := decoder.Decode(&payload); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request payload: "+err.Error())
		return
	}

	// Validate status strictly
	if payload.Status == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "status is required")
		return
	}
	statusStr, ok := payload.Status.(string)
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "status must be a string")
		return
	}
	trimmedStatus := strings.TrimSpace(statusStr)
	if trimmedStatus != model.CampaignStatusActive && trimmedStatus != model.CampaignStatusPaused {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "status must be either 'active' or 'paused'")
		return
	}

	// Update status via CampaignService
	camp, err := h.service.UpdateStatus(r.Context(), id, trimmedStatus)
	if err != nil {
		if errors.Is(err, service.ErrCampaignNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "campaign not found")
			return
		}
		if errors.Is(err, service.ErrInvalidStatus) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to update campaign: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, camp)
}

// GetByID handles GET /campaigns/:id
func (h *CampaignHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid campaign id, must be a positive integer")
		return
	}

	camp, err := h.service.GetCampaign(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrCampaignNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "campaign not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to get campaign: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, camp)
}

// List handles GET /campaigns
func (h *CampaignHandler) List(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.service.ListCampaigns(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to list campaigns: "+err.Error())
		return
	}
	if campaigns == nil {
		campaigns = []*model.Campaign{}
	}

	WriteJSON(w, http.StatusOK, campaigns)
}

type updateCampaignFullPayload struct {
	Name                 any `json:"name"`
	DailyImpressionLimit any `json:"daily_impression_limit"`
	Status               any `json:"status"`
}

// Update handles PUT /campaigns/:id (full campaign update)
func (h *CampaignHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid campaign id, must be a positive integer")
		return
	}

	if r.Body == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "request body is required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()

	var payload updateCampaignFullPayload
	if err := decoder.Decode(&payload); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request payload: "+err.Error())
		return
	}

	// 1. Validate name
	if payload.Name == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "name is required")
		return
	}
	nameStr, ok := payload.Name.(string)
	if !ok || strings.TrimSpace(nameStr) == "" {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "name must be a non-empty string")
		return
	}
	name := strings.TrimSpace(nameStr)

	// 2. Validate daily_impression_limit
	if payload.DailyImpressionLimit == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "daily_impression_limit is required")
		return
	}
	num, ok := payload.DailyImpressionLimit.(json.Number)
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "daily_impression_limit must be a number, string values are not allowed")
		return
	}
	rawLimit := num.String()
	if strings.Contains(rawLimit, ".") || strings.ContainsAny(rawLimit, "eE") {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "daily_impression_limit must be an integer, floats are not allowed")
		return
	}
	limitVal, err := num.Int64()
	if err != nil || limitVal <= 0 || limitVal > math.MaxInt32 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "daily_impression_limit must be a positive integer")
		return
	}

	// 3. Validate status
	if payload.Status == nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "status is required")
		return
	}
	statusStr, ok := payload.Status.(string)
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "status must be a string")
		return
	}
	trimmedStatus := strings.TrimSpace(statusStr)
	if trimmedStatus != model.CampaignStatusActive && trimmedStatus != model.CampaignStatusPaused {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "status must be either 'active' or 'paused'")
		return
	}

	// 4. Update via CampaignService
	camp, err := h.service.UpdateCampaign(r.Context(), id, name, int(limitVal), trimmedStatus)
	if err != nil {
		if errors.Is(err, service.ErrCampaignNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "campaign not found")
			return
		}
		if errors.Is(err, service.ErrInvalidName) || errors.Is(err, service.ErrInvalidLimit) || errors.Is(err, service.ErrInvalidStatus) {
			WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to update campaign: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, camp)
}

// Delete handles DELETE /campaigns/:id
func (h *CampaignHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid campaign id, must be a positive integer")
		return
	}

	if err := h.service.DeleteCampaign(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrCampaignNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "campaign not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to delete campaign: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
