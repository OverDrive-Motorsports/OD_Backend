/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## get_health.usecase.go - Use case returning the auth-service health status.	##
*/
package usecases

import "overdrive/services/auth-service/src/core/domain"

type GetHealthUseCase struct {
	serviceName string
}

func NewGetHealthUseCase(serviceName string) *GetHealthUseCase {
	return &GetHealthUseCase{serviceName: serviceName}
}

func (u *GetHealthUseCase) Execute() domain.HealthStatus {
	return domain.HealthStatus{
		Status:  "ok",
		Service: u.serviceName,
	}
}
