/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.service.go - Core port defining the ingestion-service health use case contract.
	##
*/
package ports

import "overdrive/services/ingestion-service/src/core/domain"

type HealthUseCase interface {
	Execute() domain.HealthStatus
}
