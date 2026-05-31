// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"context"
	"net/http"
	"overdrive/gateway/internal/core/usecases"
	"time"
)

type HealthHandler struct {
	checkHealth usecases.CheckHealthUseCase
}

func NewHealthHandler(checkHealth usecases.CheckHealthUseCase) HealthHandler {
	return HealthHandler{checkHealth: checkHealth}
}

func (h HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	status, ok := h.checkHealth.Execute(ctx)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, status)
		return
	}

	writeJSON(w, http.StatusOK, status)
}
