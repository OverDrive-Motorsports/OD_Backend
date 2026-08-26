/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## championship_dataset_middleware_test.go - Unit tests for ValidateChampionshipDataset.
 ##
 */

package httpinbound

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateChampionshipDataset(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := ValidateChampionshipDataset(next)

	tests := []struct {
		name       string
		path       string
		query      string
		wantStatus int
	}{
		{
			name:       "allowed dataset passes",
			path:       "/v1/championship/sessions/1/datasets/starting_grid",
			wantStatus: http.StatusOK,
		},
		{
			name:       "unknown dataset is rejected",
			path:       "/v1/championship/sessions/1/datasets/not-a-dataset",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-numeric driver number in path is rejected",
			path:       "/v1/championship/drivers/abc/profile",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid driver number in path passes",
			path:       "/v1/championship/drivers/44/profile",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-numeric driverNumber query is rejected",
			path:       "/v1/championship/standings",
			query:      "driverNumber=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unrelated path is passed through untouched",
			path:       "/v1/championship/something-else",
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

func TestValidateChampionshipDataset_IgnoresNonGet(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := ValidateChampionshipDataset(next)

	req := httptest.NewRequest(http.MethodPost, "/v1/championship/sessions/1/datasets/not-a-dataset", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (non GET should bypass validation)", rec.Code, http.StatusOK)
	}
}
