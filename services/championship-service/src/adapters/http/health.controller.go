/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.controller.go - Package httpadapter source file for services/championship-service/src/adapters/http.
	##
*/
package httpadapter

import (
	"encoding/json"
	"net/http"

	"overdrive/services/championship-service/src/core/ports"
)

type HealthController struct {
	usecase ports.HealthUseCase
}

// NewHealthController builds and returns a health controller with its required dependencies.
func NewHealthController(usecase ports.HealthUseCase) *HealthController {
	return &HealthController{usecase: usecase}
}

// GetHealth returns the requested health payload for the supplied identifiers.
func (c *HealthController) GetHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, c.usecase.Execute())
}

// writeJSON serializes a payload as JSON and writes it with the provided HTTP status.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
