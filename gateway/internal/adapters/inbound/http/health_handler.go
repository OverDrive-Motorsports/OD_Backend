/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## health_handler.go - Handles gateway health endpoint responses.
 ##
 */

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"context"
	"net/http"
	"overdrive/gateway/internal/core/usecases"
	"time"

	"overdrive/shared/apierror"
)

type HealthHandler struct {
	checkHealth usecases.CheckHealthUseCase
}

func NewHealthHandler(checkHealth usecases.CheckHealthUseCase) HealthHandler {
	return HealthHandler{checkHealth: checkHealth}
}

func (h HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apierror.Write(w, r.URL.Path, apierror.MethodNotAllowed("method not allowed", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	status, ok := h.checkHealth.Execute(ctx)
	if !ok {
		apierror.Write(w, r.URL.Path, apierror.ServiceUnavailable("service unavailable", nil))
		return
	}

	writeJSON(w, http.StatusOK, status)
}
