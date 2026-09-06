/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## get_health_usecase_test.go - Package usecases source file for services/auth-service/src/core/usecases.
	##
*/

package usecases

import "testing"

// TestGetHealthUseCase_Execute proves Execute always reports "ok" for the configured service
// name - there is no dependency for this trivial usecase to fail against.
func TestGetHealthUseCase_Execute(t *testing.T) {
	uc := NewGetHealthUseCase("auth-service")

	status := uc.Execute()

	if status.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", status.Status)
	}
	if status.Service != "auth-service" {
		t.Fatalf("expected service 'auth-service', got %q", status.Service)
	}
}
