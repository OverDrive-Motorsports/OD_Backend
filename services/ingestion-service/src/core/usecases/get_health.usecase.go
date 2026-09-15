/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## get_health.usecase.go - Package usecases source file for services/ingestion-service/src/core/usecases.
	##
*/
package usecases

import "overdrive/services/ingestion-service/src/core/domain"

type GetHealthUseCase struct {
	serviceName string
}

// NewGetHealthUseCase builds and returns a get health use case with its required dependencies.
func NewGetHealthUseCase(serviceName string) *GetHealthUseCase {
	return &GetHealthUseCase{serviceName: serviceName}
}

// Execute runs the use case workflow and returns the resulting domain payload.
func (u *GetHealthUseCase) Execute() domain.HealthStatus {
	return domain.HealthStatus{
		Status:  "ok",
		Service: u.serviceName,
	}
}
