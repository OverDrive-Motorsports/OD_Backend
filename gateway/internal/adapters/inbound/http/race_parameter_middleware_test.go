/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_parameter_middleware_test.go - Unit tests for ValidateRaceParameters.
##
*/

package httpinbound

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateRaceParameters(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := ValidateRaceParameters(next)

	tests := []struct {
		name       string
		path       string
		query      string
		wantStatus int
	}{
		{
			name:       "allowed session dataset passes",
			path:       "/v1/race-data/sessions/1/datasets/laps",
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown session dataset is rejected",
			path:       "/v1/race-data/sessions/1/datasets/not-a-dataset",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "allowed driver dataset passes",
			path:       "/v1/race-data/sessions/1/drivers/44/telemetry",
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown driver segment is rejected",
			path:       "/v1/race-data/sessions/1/drivers/44/not-a-dataset",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-numeric driver number in path is rejected",
			path:       "/v1/race-data/sessions/1/drivers/abc/telemetry",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-numeric driverNumber query is rejected",
			path:       "/v1/race-data/sessions/1/race/laps",
			query:      "driverNumber=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unrelated path is passed through untouched",
			path:       "/v1/race-data/something-else",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := tt.path
			if tt.query != "" {
				target += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, target, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

// TestValidateRaceParameters_MethodGate proves the method allow-list: GET/POST are proxied
// everywhere, PUT is proxied only on the exact driver broadcast route (and still goes through
// the driverNumber validation), and every other method/path combination is rejected at the
// edge with a 405 METHOD_NOT_ALLOWED envelope before reaching the upstream.
func TestValidateRaceParameters_MethodGate(t *testing.T) {
	handler := ValidateRaceParameters(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"PUT on driver broadcast is proxied", http.MethodPut, "/v1/race-data/sessions/s1/drivers/44/broadcast", http.StatusOK},
		{"PUT on driver broadcast still validates driverNumber", http.MethodPut, "/v1/race-data/sessions/s1/drivers/abc/broadcast", http.StatusBadRequest},
		{"PUT on session broadcast pass-through is 405", http.MethodPut, "/v1/race-data/sessions/s1/broadcast", http.StatusMethodNotAllowed},
		{"PUT on a driver dataset is 405", http.MethodPut, "/v1/race-data/sessions/s1/drivers/44/laps", http.StatusMethodNotAllowed},
		{"PUT on a deeper broadcast-suffixed path is 405", http.MethodPut, "/v1/race-data/sessions/s1/drivers/44/laps/broadcast", http.StatusMethodNotAllowed},
		{"PUT with empty sessionId is 405", http.MethodPut, "/v1/race-data/sessions//drivers/44/broadcast", http.StatusMethodNotAllowed},
		{"POST on race control is still proxied", http.MethodPost, "/v1/race-data/sessions/s1/race/control", http.StatusOK},
		{"DELETE is 405", http.MethodDelete, "/v1/race-data/sessions/s1/drivers/44/broadcast", http.StatusMethodNotAllowed},
		{"PATCH is 405", http.MethodPatch, "/v1/race-data/sessions/s1/datasets/laps", http.StatusMethodNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantStatus == http.StatusMethodNotAllowed && !strings.Contains(rec.Body.String(), `"METHOD_NOT_ALLOWED"`) {
				t.Fatalf("expected METHOD_NOT_ALLOWED envelope, got %s", rec.Body.String())
			}
		})
	}
}

func TestIsPositiveInt(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"1", true},
		{"42", true},
		{"0", false},
		{"-1", false},
		{"abc", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := isPositiveInt(tt.raw); got != tt.want {
			t.Errorf("isPositiveInt(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}
