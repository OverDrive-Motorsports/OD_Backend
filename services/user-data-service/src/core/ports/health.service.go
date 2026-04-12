/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.service.go - Core port defining the user-data-service health use case contract.
	##
*/
package ports

import "overdrive/services/user-data-service/src/core/domain"

type HealthUseCase interface {
	Execute() domain.HealthStatus
}
