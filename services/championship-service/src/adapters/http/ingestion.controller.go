/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.controller.go - Package httpadapter source file for services/championship-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"encoding/json"
	"net/http"

	"overdrive/services/championship-service/src/core/ports"
	"overdrive/shared/apierror"
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
// The batch payload is validated up front by the usecase (batch id, datasets, target service),
// so any error returned by Execute is treated as a validation failure of the caller-supplied
// batch rather than an internal fault.
func (c *IngestionController) ReceiveBatch(w http.ResponseWriter, r *http.Request) {
	var batch contracts.Batch
	if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid request body", err))
		return
	}

	ack, err := c.usecase.Execute(r.Context(), batch)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid ingestion batch", err))
		return
	}

	writeJSON(w, http.StatusAccepted, ack)
}
