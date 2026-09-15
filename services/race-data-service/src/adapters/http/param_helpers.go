/**
##
## OverDrive 2026
## All Technical rights reserved
##
## param_helpers.go - Shared query/path parameter parsing helpers for services/race-data-service/src/adapters/http.
##
*/

package httpadapter

import (
	"net/http"
	"strconv"
	"strings"

	"overdrive/shared/apierror"
)

// parseOptionalPositiveInt reads an optional positive-integer query parameter.
// It writes a 400 response and returns ok=false when the parameter is present
// but not a valid positive integer.
func parseOptionalPositiveInt(w http.ResponseWriter, r *http.Request, name string) (*int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid "+name+" parameter", err))
		return nil, false
	}
	return &value, true
}

// parsePathPositiveInt reads a required positive-integer path parameter.
// It writes a 400 response and returns ok=false when missing or invalid.
func parsePathPositiveInt(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	raw := strings.TrimSpace(r.PathValue(name))
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid "+name+" parameter", err))
		return 0, false
	}
	return value, true
}
