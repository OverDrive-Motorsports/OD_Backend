/**
##
## OverDrive 2026
## All Technical rights reserved
##
## get_health_test.go - Package usecases source file for services/ingestion-service/src/core/usecases.
##
*/

package usecases

import "testing"

// TestGetHealthUseCase_Execute proves Execute always reports "ok" for the configured service
// name, unconditionally (no dependency to fail against, unlike auth-service/user-data-service's
// equivalent).
func TestGetHealthUseCase_Execute(t *testing.T) {
	uc := NewGetHealthUseCase("ingestion-service")

	got := uc.Execute()

	if got.Status != "ok" {
		t.Fatalf("Status = %q, want ok", got.Status)
	}
	if got.Service != "ingestion-service" {
		t.Fatalf("Service = %q, want ingestion-service", got.Service)
	}
}
