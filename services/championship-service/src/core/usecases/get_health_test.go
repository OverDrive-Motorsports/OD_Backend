/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## get_health_test.go - Package usecases source file for services/championship-service/src/core/usecases.
	##
*/

package usecases

import "testing"

// TestGetHealthUseCase_Execute proves the health payload always reports status "ok" and echoes
// back the service name it was constructed with.
func TestGetHealthUseCase_Execute(t *testing.T) {
	uc := NewGetHealthUseCase("championship-service")
	got := uc.Execute()
	if got.Status != "ok" {
		t.Fatalf("Status = %q, want %q", got.Status, "ok")
	}
	if got.Service != "championship-service" {
		t.Fatalf("Service = %q, want %q", got.Service, "championship-service")
	}
}
