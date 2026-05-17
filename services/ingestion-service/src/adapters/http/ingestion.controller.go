/*
*

	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.controller.go - Package httpadapter source file for services/ingestion-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"

	"overdrive/services/ingestion-service/src/core/domain"
	"overdrive/services/ingestion-service/src/core/ports"
)

type IngestionController struct {
	usecase ports.OpenF1IngestionUseCase
}

// NewIngestionController builds and returns a ingestion controller with its required dependencies.
func NewIngestionController(usecase ports.OpenF1IngestionUseCase) *IngestionController {
	return &IngestionController{usecase: usecase}
}

// TriggerOpenF1Ingestion handles manual OpenF1 ingestion requests and returns the batch result.
func (c *IngestionController) TriggerOpenF1Ingestion(w http.ResponseWriter, r *http.Request) {
	var request domain.OpenF1IngestionRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := c.usecase.Execute(r.Context(), request)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, r.Context().Err()) {
			status = http.StatusRequestTimeout
		} else if request.MeetingKey <= 0 || request.SessionKey <= 0 {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

// writeError serializes an error response with the provided HTTP status code.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
