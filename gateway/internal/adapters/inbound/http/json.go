/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## json.go - Provides JSON response helper utilities for HTTP handlers.
 ##
 */

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
