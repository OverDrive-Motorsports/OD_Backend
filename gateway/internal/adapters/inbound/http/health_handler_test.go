/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## health_handler_test.go - Unit tests for HealthHandler.
 ##
 */

package httpinbound

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"overdrive/gateway/internal/core/usecases"
)

func TestHealthHandler_HandleHealth(t *testing.T) {
	handler := NewHealthHandler(usecases.NewCheckHealthUseCase())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.HandleHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body status = %q, want %q", body["status"], "ok")
	}
}

func TestHealthHandler_RejectsNonGet(t *testing.T) {
	handler := NewHealthHandler(usecases.NewCheckHealthUseCase())

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	handler.HandleHealth(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
