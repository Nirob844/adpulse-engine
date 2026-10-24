package handler

import (
	"errors"
	"net/http"

	"github.com/nirob844/adpulse-engine/internal/service"
)

// ReportHandler handles reporting and analytics HTTP requests.
type ReportHandler struct {
	service *service.ReportService
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(s *service.ReportService) *ReportHandler {
	return &ReportHandler{service: s}
}

// GetReport handles GET /report?from=&to=&group_by=campaign|day
// Returns aggregated metrics (impressions, clicks, CTR %) grouped by campaign or day.
func (h *ReportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	groupBy := r.URL.Query().Get("group_by")

	result, err := h.service.GenerateReport(r.Context(), from, to, groupBy)
	if err != nil {
		if errors.Is(err, service.ErrMissingDate) ||
			errors.Is(err, service.ErrInvalidDateFormat) ||
			errors.Is(err, service.ErrFromAfterTo) ||
			errors.Is(err, service.ErrInvalidGroupBy) {
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "failed to generate report: "+err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, result)
}
