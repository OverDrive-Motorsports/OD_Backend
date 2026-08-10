/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health_controller_test.go - Package httpadapter source file for services/championship-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"overdrive/services/championship-service/src/core/domain"
)

// fakeHealthUseCase implements ports.HealthUseCase for controller-level tests.
type fakeHealthUseCase struct {
	status domain.HealthStatus
}

func (f *fakeHealthUseCase) Execute() domain.HealthStatus {
	return f.status
}

// TestHealthController_GetHealth proves the handler writes a 200 with the usecase's payload
// serialized verbatim as JSON, with no transformation applied.
func TestHealthController_GetHealth(t *testing.T) {
	usecase := &fakeHealthUseCase{status: domain.HealthStatus{Status: "ok", Service: "championship-service"}}
	controller := NewHealthController(usecase)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	controller.GetHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got domain.HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response: %v (body: %s)", err, rec.Body.String())
	}
	if got != usecase.status {
		t.Fatalf("got %+v, want %+v", got, usecase.status)
	}
}
