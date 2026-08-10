/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health_controller_test.go - Package httpadapter source file for services/ingestion-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"overdrive/services/ingestion-service/src/core/domain"
)

// fakeHealthUseCase implements ports.HealthUseCase for controller-level tests.
type fakeHealthUseCase struct {
	status domain.HealthStatus
}

func (f *fakeHealthUseCase) Execute() domain.HealthStatus { return f.status }

// TestHealthController_GetHealth proves GetHealth writes a 200 with the usecase's payload
// serialized as JSON, unmodified.
func TestHealthController_GetHealth(t *testing.T) {
	want := domain.HealthStatus{Status: "ok", Service: "ingestion-service"}
	controller := NewHealthController(&fakeHealthUseCase{status: want})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	controller.GetHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var got domain.HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unexpected decode error: %v (body: %s)", err, rec.Body.String())
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestWriteJSON proves writeJSON sets the status code, the JSON content type, and serializes the
// payload verbatim - the shared helper every controller in this package relies on.
func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSON(rec, http.StatusTeapot, map[string]string{"hello": "world"})

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unexpected decode error: %v (body: %s)", err, rec.Body.String())
	}
	if got["hello"] != "world" {
		t.Fatalf("got %+v, want hello=world", got)
	}
}
