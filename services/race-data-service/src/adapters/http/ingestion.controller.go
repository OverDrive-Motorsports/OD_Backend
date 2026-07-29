/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.controller.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"encoding/json"
	"net/http"

	"overdrive/services/race-data-service/src/core/ports"
	contracts "overdrive/shared/contracts/ingestion"
)

type IngestionController struct {
	usecase ports.IngestionBatchUseCase
}

// NewIngestionController builds and returns a ingestion controller with its required dependencies.
func NewIngestionController(usecase ports.IngestionBatchUseCase) *IngestionController {
	return &IngestionController{usecase: usecase}
}

// ReceiveBatch handles an incoming HTTP ingestion batch and writes the acknowledgement response.
func (c *IngestionController) ReceiveBatch(w http.ResponseWriter, r *http.Request) {
	var batch contracts.Batch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	ack, err := c.usecase.Execute(r.Context(), batch)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusAccepted, ack)
}
