/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.entity.go - Domain entity representing the user-data-service health response payload.
	##
*/
package domain

type HealthStatus struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}
