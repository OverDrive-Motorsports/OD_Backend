/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## check_health_test.go - Unit tests for CheckHealthUseCase.
 ##
 */

package usecases

import (
	"context"
	"testing"
)

func TestCheckHealthUseCase_Execute(t *testing.T) {
	useCase := NewCheckHealthUseCase()

	status, ok := useCase.Execute(context.Background())

	if !ok {
		t.Fatal("expected ok to be true")
	}
	if status.Status != "ok" {
		t.Fatalf("expected status %q, got %q", "ok", status.Status)
	}
}
