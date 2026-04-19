/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## health.controller.go - HTTP controller exposing the ingestion-service health endpoint.
	##
*/
package httpadapter

import (
	"encoding/json"
	"net/http"

	"overdrive/services/ingestion-service/src/core/ports"
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
