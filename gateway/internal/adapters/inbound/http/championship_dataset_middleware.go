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

	"overdrive/shared/apierror"
)

var allowedChampionshipDatasets = map[string]struct{}{
	"session_result":        {},
	"starting_grid":         {},
	"championship_drivers":  {},
	"championship_teams":    {},
}

// ValidateChampionshipDataset rejects invalid dataset names and malformed
// numeric parameters (driverNumber path/query) for the public championship v1
// routes. All other paths are forwarded unchanged.
func ValidateChampionshipDataset(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		if dataset, ok := extractChampionshipDataset(r.URL.Path); ok {
			if _, allowed := allowedChampionshipDatasets[dataset]; !allowed {
				apierror.Write(w, r.URL.Path, apierror.Validation("invalid parameter", nil))
				return
			}
		}

		if driverNumber, ok := extractChampionshipDriverNumber(r.URL.Path); ok && !isPositiveInt(driverNumber) {
			apierror.Write(w, r.URL.Path, apierror.Validation("invalid parameter", nil))
			return
		}

		if driverNumber := r.URL.Query().Get("driverNumber"); driverNumber != "" && !isPositiveInt(driverNumber) {
			apierror.Write(w, r.URL.Path, apierror.Validation("invalid parameter", nil))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractChampionshipDriverNumber extracts the {driverNumber} segment from
// /v1/championship/drivers/{driverNumber}/profile.
func extractChampionshipDriverNumber(path string) (string, bool) {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "", false
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) != 5 {
		return "", false
	}

	if parts[0] != "v1" || parts[1] != "championship" || parts[2] != "drivers" || parts[4] != "profile" {
		return "", false
	}

	driverNumber := strings.TrimSpace(parts[3])
	if driverNumber == "" {
		return "", false
	}

	return driverNumber, true
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
