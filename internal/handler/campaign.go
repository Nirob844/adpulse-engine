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
	"github.com/nirob844/mini-ad-server/internal/repo"
)

// CampaignHandler handles campaign management HTTP requests.
type CampaignHandler struct {
	repo *repo.CampaignRepo
}

// NewCampaignHandler creates a new CampaignHandler.
func NewCampaignHandler(r *repo.CampaignRepo) *CampaignHandler {
	return &CampaignHandler{repo: r}
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
		writeJSONError(w, http.StatusBadRequest, "request body is required")
		return
	}
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()

	var payload createCampaignPayload
	if err := decoder.Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	// 1. Validate name (required, non-empty string)
	if payload.Name == nil {
		writeJSONError(w, http.StatusBadRequest, "name is required")
		return
	}
	nameStr, ok := payload.Name.(string)
	if !ok || strings.TrimSpace(nameStr) == "" {
		writeJSONError(w, http.StatusBadRequest, "name must be a non-empty string")
		return
	}
	name := strings.TrimSpace(nameStr)

	// 2. Validate daily_impression_limit (required positive integer; reject "5", 1.5, <= 0)
	if payload.DailyImpressionLimit == nil {
		writeJSONError(w, http.StatusBadRequest, "daily_impression_limit is required")
		return
	}
	num, ok := payload.DailyImpressionLimit.(json.Number)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "daily_impression_limit must be a number, string values are not allowed")
		return
	}
	rawLimit := num.String()
	if strings.Contains(rawLimit, ".") || strings.ContainsAny(rawLimit, "eE") {
		writeJSONError(w, http.StatusBadRequest, "daily_impression_limit must be an integer, floats are not allowed")
		return
	}
	limitVal, err := num.Int64()
	if err != nil || limitVal <= 0 || limitVal > math.MaxInt32 {
		writeJSONError(w, http.StatusBadRequest, "daily_impression_limit must be a positive integer")
		return
	}

	// 3. Validate status (optional, default: "active", allowed: "active" | "paused")
	status := model.CampaignStatusActive
	if payload.Status != nil {
		statusStr, ok := payload.Status.(string)
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "status must be a string")
			return
		}
		trimmedStatus := strings.TrimSpace(statusStr)
		if trimmedStatus != model.CampaignStatusActive && trimmedStatus != model.CampaignStatusPaused {
			writeJSONError(w, http.StatusBadRequest, "status must be either 'active' or 'paused'")
			return
		}
		status = trimmedStatus
	}

	// 4. Insert into database
	camp, err := h.repo.Create(r.Context(), name, int(limitVal), status)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create campaign: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(camp)
}

// UpdateStatus handles PATCH /campaigns/:id
func (h *CampaignHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid campaign id, must be a positive integer")
		return
	}

	if r.Body == nil {
		writeJSONError(w, http.StatusBadRequest, "request body is required")
		return
	}
	defer r.Body.Close()

	var payload updateCampaignPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request payload: "+err.Error())
		return
	}

	// Validate status strictly
	if payload.Status == nil {
		writeJSONError(w, http.StatusBadRequest, "status is required")
		return
	}
	statusStr, ok := payload.Status.(string)
	if !ok {
		writeJSONError(w, http.StatusBadRequest, "status must be a string")
		return
	}
	trimmedStatus := strings.TrimSpace(statusStr)
	if trimmedStatus != model.CampaignStatusActive && trimmedStatus != model.CampaignStatusPaused {
		writeJSONError(w, http.StatusBadRequest, "status must be either 'active' or 'paused'")
		return
	}

	// Update status in repository
	camp, err := h.repo.UpdateStatus(r.Context(), id, trimmedStatus)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "failed to update campaign: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(camp)
}

// List handles GET /campaigns
func (h *CampaignHandler) List(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.repo.ListAll(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list campaigns: "+err.Error())
		return
	}
	if campaigns == nil {
		campaigns = []*model.Campaign{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(campaigns)
}

// writeJSONError writes an HTTP error status code and JSON error object.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
