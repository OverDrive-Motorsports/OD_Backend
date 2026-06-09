/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## championship_dataset_middleware.go - Validates championship dataset path parameter on public v1 routes.
 ##
 */

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"net/http"
	"strings"
)

var allowedChampionshipDatasets = map[string]struct{}{
	"session_result":        {},
	"starting_grid":         {},
	"championship_drivers":  {},
	"championship_teams":    {},
}

// ValidateChampionshipDataset rejects invalid dataset names only for the public
// championship v1 dataset endpoint. All other paths are forwarded unchanged.
func ValidateChampionshipDataset(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		dataset, ok := extractChampionshipDataset(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		if _, allowed := allowedChampionshipDatasets[dataset]; !allowed {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid parameter"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func extractChampionshipDataset(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", false
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 6 {
		return "", false
	}

	if parts[0] != "v1" || parts[1] != "championship" || parts[2] != "sessions" || parts[4] != "datasets" {
		return "", false
	}

	sessionID := strings.TrimSpace(parts[3])
	dataset := strings.TrimSpace(parts[5])
	if sessionID == "" || dataset == "" {
		return "", false
	}

	return dataset, true
}
