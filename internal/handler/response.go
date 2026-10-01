package handler

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse represents a standardized, structured error payload.
type ErrorResponse struct {
	Status int    `json:"status"`
	Code   string `json:"code"`
	Error  string `json:"error"`
}

// WriteJSON sets the Content-Type header, writes the status code, and serializes
// data into clean, indented JSON (2 spaces) for beautiful readability.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	_ = enc.Encode(data)
}

// WriteError writes a standardized, structured JSON error response.
func WriteError(w http.ResponseWriter, status int, code string, message string) {
	resp := ErrorResponse{
		Status: status,
		Code:   code,
		Error:  message,
	}
	WriteJSON(w, status, resp)
}
