/**
##
## OverDrive 2026
## All Technical rights reserved
##
## live.controller.go - HTTP handlers of the live ingestion jobs.
##
*/

package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"overdrive/services/ingestion-service/src/core/domain"
	"overdrive/services/ingestion-service/src/core/ports"
	"overdrive/shared/apierror"
)

// liveStartTimeout bounds the session lookup of a start, under the server write timeout.
const liveStartTimeout = 20 * time.Second

type LiveController struct {
	usecase ports.LiveIngestionUseCase
}

// NewLiveController builds and returns a live controller with its required dependencies.
func NewLiveController(usecase ports.LiveIngestionUseCase) *LiveController {
	return &LiveController{usecase: usecase}
}

// Start starts the live job of the session: 202 when it starts, 200 when it was already running.
func (c *LiveController) Start(w http.ResponseWriter, r *http.Request) {
	sessionKey, ok := readSessionKey(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), liveStartTimeout)
	defer cancel()
	status, started, err := c.usecase.Start(ctx, sessionKey)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyLiveError(err))
		return
	}
	code := http.StatusOK
	if started {
		code = http.StatusAccepted
	}
	writeJSON(w, code, status)
}

// Stop stops the live job of the session and returns its status.
func (c *LiveController) Stop(w http.ResponseWriter, r *http.Request) {
	sessionKey, ok := readSessionKey(w, r)
	if !ok {
		return
	}
	status, err := c.usecase.Stop(r.Context(), sessionKey)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyLiveError(err))
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// Status returns the status of the latest live job of the session.
func (c *LiveController) Status(w http.ResponseWriter, r *http.Request) {
	sessionKey, ok := readSessionKey(w, r)
	if !ok {
		return
	}
	status, err := c.usecase.Status(sessionKey)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyLiveError(err))
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// readSessionKey reads the path parameter, writing the 400 itself when it is not a positive int.
func readSessionKey(w http.ResponseWriter, r *http.Request) (int, bool) {
	sessionKey, err := strconv.ParseInt(r.PathValue("sessionKey"), 10, 32)
	if err != nil || sessionKey <= 0 {
		apierror.Write(w, r.URL.Path, apierror.Validation("sessionKey must be a positive integer", err))
		return 0, false
	}
	return int(sessionKey), true
}

// classifyLiveError maps a live use case error to the matching apierror.
func classifyLiveError(err error) *apierror.Error {
	switch {
	case errors.Is(err, domain.ErrLiveSessionNotFound):
		return apierror.NotFound("OPENF1_SESSION", "OpenF1 does not know this session", err)
	case errors.Is(err, domain.ErrLiveJobNotFound):
		return apierror.NotFound("LIVE_JOB", "no live job for this session", err)
	case errors.Is(err, domain.ErrLiveJobLimit):
		return apierror.Conflict("LIVE_JOB", "the maximum number of live jobs is already running", err)
	case errors.Is(err, domain.ErrLiveJobStopping):
		return apierror.Conflict("LIVE_JOB", "the live job is being stopped, retry in a few seconds", err)
	case errors.Is(err, domain.ErrSessionBusy):
		return apierror.Conflict("SESSION_INGESTION", "a manual ingestion of this session is running", err)
	case errors.Is(err, domain.ErrLiveShuttingDown):
		return apierror.ServiceUnavailable("the service is shutting down", err)
	case errors.Is(err, context.DeadlineExceeded):
		return apierror.UpstreamTimeout("OpenF1 request timed out", err)
	default:
		return apierror.UpstreamUnavailable("failed to read the session from OpenF1", err)
	}
}
