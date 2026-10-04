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
	"strings"
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

// TestValidateChampionshipDataset_MethodGate proves the method allow-list: GET is proxied
// everywhere, PUT only on the exact session broadcast route, and every other method/path
// combination is rejected at the edge with a 405 METHOD_NOT_ALLOWED envelope.
func TestValidateChampionshipDataset_MethodGate(t *testing.T) {
	handler := ValidateChampionshipDataset(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{"PUT on session broadcast is proxied", http.MethodPut, "/v1/championship/sessions/s1/broadcast", http.StatusOK},
		{"PUT on session itself is 405", http.MethodPut, "/v1/championship/sessions/s1", http.StatusMethodNotAllowed},
		{"PUT on a dataset is 405", http.MethodPut, "/v1/championship/sessions/s1/datasets/session_result", http.StatusMethodNotAllowed},
		{"PUT on a deeper broadcast-suffixed path is 405", http.MethodPut, "/v1/championship/sessions/s1/standings/broadcast", http.StatusMethodNotAllowed},
		{"PUT with empty sessionId is 405", http.MethodPut, "/v1/championship/sessions//broadcast", http.StatusMethodNotAllowed},
		{"POST is 405", http.MethodPost, "/v1/championship/sessions/s1/datasets/not-a-dataset", http.StatusMethodNotAllowed},
		{"DELETE is 405", http.MethodDelete, "/v1/championship/sessions/s1/broadcast", http.StatusMethodNotAllowed},
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
