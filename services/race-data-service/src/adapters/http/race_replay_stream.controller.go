/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## race_replay_stream.controller.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"net/http"

	"overdrive/services/race-data-service/src/core/ports"
	"overdrive/shared/apierror"
)

type RaceReplayStreamController struct {
	usecase ports.RaceReplayUseCase
}

// NewRaceReplayStreamController builds and returns a race replay controller with its required dependencies.
func NewRaceReplayStreamController(usecase ports.RaceReplayUseCase) *RaceReplayStreamController {
	return &RaceReplayStreamController{usecase: usecase}
}

// GetReplay implements GET /sessions/{sessionId}/race/replay: a plain JSON
// bulk dump of every stored telemetry/position/radio/pitStop/stint/lap
// sample for a session (grouped by driver number), plus its track-wide
// raceControl/weather samples (flat arrays). It is additive to (not a
// replacement for) the existing snapshot endpoints — see doc/endpoint.md.
func (c *RaceReplayStreamController) GetReplay(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	exists, err := c.usecase.SessionExists(r.Context(), sessionID)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.UpstreamUnavailable("failed to verify session with championship-service", err))
		return
	}
	if !exists {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}

	driverNumber, ok := parseOptionalPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}

	replay, err := c.usecase.GetReplay(r.Context(), sessionID, driverNumber)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to build race replay", err))
		return
	}

	writeJSON(w, http.StatusOK, replay)
}
