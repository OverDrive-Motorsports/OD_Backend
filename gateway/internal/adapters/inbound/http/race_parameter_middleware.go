/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## race_parameter_middleware.go - Validates race-data route parameters on public v1 routes.
 ##
 */

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"net/http"
	"strconv"
	"strings"
)

var allowedRaceSessionDatasets = map[string]struct{}{
	"laps":                 {},
	"car_data":             {},
	"telemetry":            {},
	"location":             {},
	"position":             {},
	"intervals":            {},
	"stints":               {},
	"pit":                  {},
	"pit_stops":            {},
	"weather":              {},
	"team_radio":           {},
	"overtakes":            {},
	"race_control":         {},
	"session_result":       {},
	"starting_grid":        {},
	"championship_drivers": {},
	"championship_teams":   {},
}

var allowedRaceDriverDatasets = map[string]struct{}{
	"laps":      {},
	"telemetry": {},
	"location":  {},
	"position":  {},
	"intervals": {},
	"stints":    {},
	"pit":       {},
	"radio":     {},
	"result":    {},
}

// These are dedicated driver endpoints sharing the same 7-segment path shape
// as /drivers/{driverNumber}/{dataset}.
var allowedRaceDriverActions = map[string]struct{}{
	"profile":   {},
	"broadcast": {},
}

// ValidateRaceParameters validates selected public /v1/race-data parameters at the gateway edge.
func ValidateRaceParameters(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		parts := splitPath(r.URL.Path)
		if len(parts) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		if dataset, ok := extractRaceSessionDataset(parts); ok {
			if _, allowed := allowedRaceSessionDatasets[dataset]; !allowed {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid parameter"})
				return
			}
		}

		if segment, ok := extractRaceDriverSegment(parts); ok {
			if !isAllowedRaceDriverSegment(segment) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid parameter"})
				return
			}
		}

		if driverNumber, ok := extractRaceDriverNumber(parts); ok && !isPositiveInt(driverNumber) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid parameter"})
			return
		}

		if lapNumber, ok := extractRaceLapNumber(parts); ok && !isPositiveInt(lapNumber) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid parameter"})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "/")
}

func extractRaceSessionDataset(parts []string) (string, bool) {
	if len(parts) != 6 {
		return "", false
	}

	if parts[0] != "v1" || parts[1] != "race-data" || parts[2] != "sessions" || parts[4] != "datasets" {
		return "", false
	}

	if strings.TrimSpace(parts[3]) == "" || strings.TrimSpace(parts[5]) == "" {
		return "", false
	}

	return strings.TrimSpace(parts[5]), true
}

func extractRaceDriverSegment(parts []string) (string, bool) {
	if len(parts) != 7 {
		return "", false
	}

	if parts[0] != "v1" || parts[1] != "race-data" || parts[2] != "sessions" || parts[4] != "drivers" {
		return "", false
	}

	if strings.TrimSpace(parts[3]) == "" || strings.TrimSpace(parts[5]) == "" {
		return "", false
	}

	segment := strings.TrimSpace(parts[6])
	if segment == "" {
		return "", false
	}

	return segment, true
}

func isAllowedRaceDriverSegment(segment string) bool {
	if _, allowed := allowedRaceDriverDatasets[segment]; allowed {
		return true
	}

	_, allowed := allowedRaceDriverActions[segment]
	return allowed
}

func extractRaceDriverNumber(parts []string) (string, bool) {
	if len(parts) < 7 {
		return "", false
	}

	if parts[0] != "v1" || parts[1] != "race-data" || parts[2] != "sessions" || parts[4] != "drivers" {
		return "", false
	}

	if strings.TrimSpace(parts[3]) == "" {
		return "", false
	}

	driverNumber := strings.TrimSpace(parts[5])
	if driverNumber == "" {
		return "", false
	}

	return driverNumber, true
}

func extractRaceLapNumber(parts []string) (string, bool) {
	if len(parts) != 9 {
		return "", false
	}

	if parts[0] != "v1" ||
		parts[1] != "race-data" ||
		parts[2] != "sessions" ||
		parts[4] != "drivers" ||
		parts[6] != "laps" ||
		parts[8] != "location" {
		return "", false
	}

	if strings.TrimSpace(parts[3]) == "" || strings.TrimSpace(parts[5]) == "" {
		return "", false
	}

	lapNumber := strings.TrimSpace(parts[7])
	if lapNumber == "" {
		return "", false
	}

	return lapNumber, true
}

func isPositiveInt(raw string) bool {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return false
	}

	return value > 0
}
