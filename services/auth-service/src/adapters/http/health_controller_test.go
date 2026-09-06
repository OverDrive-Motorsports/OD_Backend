/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health_controller_test.go - Package httpadapter source file for services/auth-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// fakeHealthUseCase implements ports.HealthUseCase for HealthController tests.
type fakeHealthUseCase struct {
	status domain.HealthStatus
}

func (f *fakeHealthUseCase) Execute() domain.HealthStatus {
	return f.status
}

// TestHealthController_GetHealth proves GetHealth writes 200 with the usecase's status
// serialized as JSON.
func TestHealthController_GetHealth(t *testing.T) {
	usecase := &fakeHealthUseCase{status: domain.HealthStatus{Status: "ok", Service: "auth-service"}}
	controller := NewHealthController(usecase)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	controller.GetHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var got domain.HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if got.Status != "ok" || got.Service != "auth-service" {
		t.Fatalf("unexpected health status: %+v", got)
	}
}
