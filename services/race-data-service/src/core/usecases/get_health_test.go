/**
##
## OverDrive 2026
## All Technical rights reserved
##
## get_health_test.go - Package usecases source file for services/race-data-service/src/core/usecases.
##
*/

package usecases

import "testing"

// TestGetHealthUseCase_Execute proves the trivial health payload carries the configured service
// name and a fixed "ok" status.
func TestGetHealthUseCase_Execute(t *testing.T) {
	uc := NewGetHealthUseCase("race-data-service")
	got := uc.Execute()
	if got.Status != "ok" || got.Service != "race-data-service" {
		t.Fatalf("unexpected health payload: %+v", got)
	}
}
