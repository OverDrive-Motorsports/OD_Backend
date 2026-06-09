/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## health.go - Defines health domain models used by gateway use cases.
 ##
 */

// Package domain declares gateway core domain models.

package domain

type HealthStatus struct {
	Status string `json:"status"`
}
