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
