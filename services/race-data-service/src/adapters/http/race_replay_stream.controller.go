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
	"bufio"
	"errors"
	"log"
	"net/http"
	"strconv"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
	"overdrive/shared/apierror"
)

// replayBufferBytes is the write buffer of the streamed replay response.
const replayBufferBytes = 64 << 10

// replayRetryAfterSeconds is the Retry-After sent when the concurrent replay cap is reached.
const replayRetryAfterSeconds = 10

type RaceReplayStreamController struct {
	usecase ports.RaceReplayUseCase
}

// NewRaceReplayStreamController builds and returns a race replay controller with its required dependencies.
func NewRaceReplayStreamController(usecase ports.RaceReplayUseCase) *RaceReplayStreamController {
	return &RaceReplayStreamController{usecase: usecase}
}

// GetReplay implements GET /sessions/{sessionId}/race/replay: a plain JSON bulk dump of every
// stored telemetry/position/radio/pitStop/stint/lap sample for a session (grouped by driver
// number), plus its track-wide raceControl/weather samples (flat arrays). The object is
// streamed part by part, so memory stays bounded whatever the session size. Optional query
// parameters: driverNumber, fromUtc and toUtc (RFC 3339, inclusive).
//
// The status is sent with the first part: an error before it gets a normal error response, an
// error after it aborts the connection so a truncated dump never looks complete.
func (c *RaceReplayStreamController) GetReplay(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionId")
	driverNumber, ok := parseOptionalPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	from, ok := parseOptionalTime(w, r, "fromUtc")
	if !ok {
		return
	}
	to, ok := parseOptionalTime(w, r, "toUtc")
	if !ok {
		return
	}

	exists, err := c.usecase.SessionExists(r.Context(), sessionID)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.UpstreamUnavailable("failed to verify session with championship-service", err))
		return
	}
	if !exists {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}

	buffer := bufio.NewWriterSize(w, replayBufferBytes)
	dump := newReplayJSONWriter(buffer)
	started := false
	emit := func(part domain.ReplayPart) error {
		if !started {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			started = true
		}
		return dump.write(part)
	}

	query := domain.ReplayQuery{DriverNumber: driverNumber, Window: domain.TimeWindow{From: from, To: to}}
	err = c.usecase.StreamReplay(r.Context(), sessionID, query, emit)
	if err == nil {
		if !started {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
		}
		err = dump.finish()
		if err == nil {
			err = buffer.Flush()
		}
		if err != nil {
			log.Printf("replay %s: writing the response failed: %v", strconv.Quote(sessionID), err)
		}
		return
	}
	if started {
		log.Printf("replay %s: aborted after the response started: %v", strconv.Quote(sessionID), err)
		panic(http.ErrAbortHandler)
	}
	switch {
	case errors.Is(err, domain.ErrReplayBusy):
		w.Header().Set("Retry-After", strconv.Itoa(replayRetryAfterSeconds))
		apierror.Write(w, r.URL.Path, apierror.ServiceUnavailable("too many replays in progress, retry later", err))
	case errors.Is(err, domain.ErrInvalidTimeWindow):
		apierror.Write(w, r.URL.Path, apierror.Validation("fromUtc must not be after toUtc", err))
	default:
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to build race replay", err))
	}
}
