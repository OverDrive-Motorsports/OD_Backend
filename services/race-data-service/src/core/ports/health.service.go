/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.service.go - Package ports source file for services/race-data-service/src/core/ports.
	##
*/
package ports

import "overdrive/services/race-data-service/src/core/domain"

type HealthUseCase interface {
	Execute() domain.HealthStatus
}
