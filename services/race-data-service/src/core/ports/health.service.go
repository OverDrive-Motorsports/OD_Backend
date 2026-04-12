/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.service.go - Core port defining the race-data-service health use case contract.
	##
*/
package ports

import "overdrive/services/race-data-service/src/core/domain"

type HealthUseCase interface {
	Execute() domain.HealthStatus
}
