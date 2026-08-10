/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health_controller_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"overdrive/services/race-data-service/src/core/domain"
)

// fakeHealthUseCase implements ports.HealthUseCase.
type fakeHealthUseCase struct {
	status domain.HealthStatus
}

func (f *fakeHealthUseCase) Execute() domain.HealthStatus { return f.status }

// TestHealthController_GetHealth proves the handler writes a 200 with the usecase's payload
// unmodified - this endpoint has no error path (see od-backend-expert.md's apierror migration
// notes), so there is nothing else to branch on.
func TestHealthController_GetHealth(t *testing.T) {
	c := NewHealthController(&fakeHealthUseCase{status: domain.HealthStatus{Status: "ok", Service: "race-data-service"}})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	c.GetHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got domain.HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if got.Status != "ok" || got.Service != "race-data-service" {
		t.Fatalf("unexpected health payload: %+v", got)
	}
}
