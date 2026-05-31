/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## check_health.go - Use case returning gateway health status payload.
 ##
 */

// Package usecases contains gateway application use cases.

package usecases

import (
	"context"
	"overdrive/gateway/internal/core/domain"
)

type CheckHealthUseCase struct{}

func NewCheckHealthUseCase() CheckHealthUseCase {
	return CheckHealthUseCase{}
}

func (u CheckHealthUseCase) Execute(_ context.Context) (domain.HealthStatus, bool) {
	return domain.HealthStatus{Status: "ok"}, true
}
