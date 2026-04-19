/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.controller.go - HTTP controller exposing the championship-service health endpoint.
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

func NewHealthController(usecase ports.HealthUseCase) *HealthController {
	return &HealthController{usecase: usecase}
}

func (c *HealthController) GetHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, c.usecase.Execute())
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
