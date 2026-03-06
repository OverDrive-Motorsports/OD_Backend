/**
##
## OverDrive 2026
## All Technical rights reserved
##
## response.go - Shared JSON response helpers for success and error payloads.
##
*/

package api

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

// writeJSON serializes a JSON payload with the provided HTTP status code.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError serializes a standardized JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}
