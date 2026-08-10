/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## ingestion.controller.go - Package httpadapter source file for services/ingestion-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"overdrive/services/ingestion-service/src/core/domain"
	"overdrive/services/ingestion-service/src/core/ports"
	"overdrive/shared/apierror"
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
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid request body", err))
		return
	}

	result, err := c.usecase.Execute(r.Context(), request)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyIngestionError(err, r.Context(), request))
		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

// classifyIngestionError maps an ingestion usecase error to the matching apierror. Invalid
// caller-supplied identifiers and unsupported OpenF1 resources are both classified as
// validation errors (400); a context deadline is reported as an upstream timeout (504);
// anything else is treated as an upstream OpenF1/dispatch failure (502), since Execute only
// returns errors once request shape has already been checked (see IngestOpenF1UseCase.Execute).
func classifyIngestionError(err error, ctx context.Context, request domain.OpenF1IngestionRequest) *apierror.Error {
	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return apierror.UpstreamTimeout("OpenF1 request timed out", err)
	}

	if request.MeetingKey <= 0 || request.SessionKey <= 0 {
		return apierror.Validation("meetingKey and sessionKey must be greater than 0", err)
	}

	if errors.Is(err, domain.ErrUnsupportedResource) {
		return apierror.Validation("unsupported OpenF1 resource requested", err)
	}

	return apierror.UpstreamUnavailable("failed to ingest data from OpenF1", err)
}
