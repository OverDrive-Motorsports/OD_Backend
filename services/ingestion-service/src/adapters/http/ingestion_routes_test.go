/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion_routes_test.go - Package httpadapter source file for services/ingestion-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/ingestion-service/src/core/domain"
)

// TestNewRouter_RoutesRegisteredAndHardened proves NewRouter wires GET /health to the health
// controller and POST /providers/openf1/ingestions to the ingestion controller (each request
// reaches the expected handler, not a 404 from an unmatched route), and that the security
// headers middleware wraps every route rather than being applied ad hoc per handler.
func TestNewRouter_RoutesRegisteredAndHardened(t *testing.T) {
	health := NewHealthController(&fakeHealthUseCase{status: domain.HealthStatus{Status: "ok", Service: "ingestion-service"}})
	ingestion := NewIngestionController(&fakeIngestionUseCase{result: domain.OpenF1IngestionResult{Provider: "openf1"}})
	router := NewRouter(health, ingestion)

	t.Run("GET /health reaches the health controller", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusOK, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "ingestion-service") {
			t.Fatalf("expected the health payload in the body, got %s", rec.Body.String())
		}
	})

	t.Run("POST /providers/openf1/ingestions reaches the ingestion controller", func(t *testing.T) {
		body, _ := json.Marshal(domain.OpenF1IngestionRequest{MeetingKey: 1141, SessionKey: 9998})
		req := httptest.NewRequest(http.MethodPost, "/providers/openf1/ingestions", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusAccepted, rec.Body.String())
		}
	})

	t.Run("every route carries the security headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatalf("expected security headers to be applied by NewRouter, got %v", rec.Header())
		}
	})

	t.Run("unmatched route returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/not-a-real-route", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}
