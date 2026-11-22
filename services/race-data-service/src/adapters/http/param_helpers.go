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
	"fmt"
	"net/http"
	"overdrive/services/race-data-service/src/core/domain"
	"strconv"
	"strings"
	"time"

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

// parseOptionalTime reads an optional RFC 3339 timestamp query parameter (for example
// fromUtc=YYYY-MM-DDThh:mm:ssZ). It writes a 400 response and returns ok=false when invalid.
func parseOptionalTime(w http.ResponseWriter, r *http.Request, name string) (*time.Time, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid "+name+" parameter, expected an RFC 3339 timestamp", err))
		return nil, false
	}
	return &value, true
}

// NextOffsetHeader tells the client a paged list has more rows and where the next page starts.
const NextOffsetHeader = "X-Next-Offset"

// parsePage reads the optional limit (1 to domain.MaxPageLimit, default the maximum) and offset
// (0 or more, default 0) query parameters. It writes a 400 response and returns ok=false when
// either is invalid.
func parsePage(w http.ResponseWriter, r *http.Request) (domain.Page, bool) {
	page := domain.DefaultPage()
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > domain.MaxPageLimit {
			apierror.Write(w, r.URL.Path, apierror.Validation(fmt.Sprintf("invalid limit parameter, expected 1 to %d", domain.MaxPageLimit), err))
			return domain.Page{}, false
		}
		page.Limit = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			apierror.Write(w, r.URL.Path, apierror.Validation("invalid offset parameter, expected 0 or more", err))
			return domain.Page{}, false
		}
		page.Offset = value
	}
	return page, true
}

// setNextOffset sets NextOffsetHeader when the list continues on another page.
func setNextOffset(w http.ResponseWriter, next *int) {
	if next != nil {
		w.Header().Set(NextOffsetHeader, strconv.Itoa(*next))
	}
}
