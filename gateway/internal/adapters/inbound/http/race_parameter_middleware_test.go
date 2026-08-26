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

func TestValidateRaceParameters_IgnoresNonGetPost(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := ValidateRaceParameters(next)

	req := httptest.NewRequest(http.MethodDelete, "/v1/race-data/sessions/1/datasets/not-a-dataset", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (non GET/POST should bypass validation)", rec.Code, http.StatusOK)
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
