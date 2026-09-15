/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.entity.go - Package domain source file for services/ingestion-service/src/core/domain.
	##
*/
package domain

type HealthStatus struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}
