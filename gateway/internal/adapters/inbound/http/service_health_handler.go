/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## service_health_handler.go - Probes upstream service health endpoints with load balancing.
 ##
 */

// Package httpinbound contains inbound HTTP handlers, middleware, and proxy adapters.

package httpinbound

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"overdrive/shared/apierror"
)

type ServiceHealthHandler struct {
	serviceName string
	targets     []*url.URL
	counter     uint64
	client      *http.Client
}

func NewServiceHealthHandler(serviceName string, targets []*url.URL) *ServiceHealthHandler {
	return &ServiceHealthHandler{
		serviceName: serviceName,
		targets:     targets,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

func (h *ServiceHealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apierror.Write(w, r.URL.Path, apierror.MethodNotAllowed("method not allowed", nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	target := h.nextTarget()
	requestURL := *target
	requestURL.Path = joinPaths(target.Path, "/health")
	requestURL.RawPath = requestURL.EscapedPath()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("internal error", err))
		return
	}

	resp, err := h.client.Do(req)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.ServiceUnavailable(fmt.Sprintf("service unavailable: %s", h.serviceName), err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": h.serviceName})
		return
	}

	apierror.Write(w, r.URL.Path, apierror.ServiceUnavailable(
		fmt.Sprintf("service unavailable: %s (upstream status %d)", h.serviceName, resp.StatusCode),
		nil,
	))
}

func (h *ServiceHealthHandler) nextTarget() *url.URL {
	index := atomic.AddUint64(&h.counter, 1)
	return h.targets[(index-1)%uint64(len(h.targets))]
}
