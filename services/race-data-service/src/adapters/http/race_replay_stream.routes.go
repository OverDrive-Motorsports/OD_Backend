/**
##
## OverDrive 2026
## All Technical rights reserved
##
## race_replay_stream.routes.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
##
*/

package httpadapter

import (
	"net/http"
	"time"

	"overdrive/shared/bootstrap"
)

// replayWriteTimeout lets the streamed replay dump outlive the server WriteTimeout: a full race
// is tens of megabytes and a mobile client may read it slowly.
const replayWriteTimeout = 10 * time.Minute

// registerRaceReplayStreamRoutes attaches this module's HTTP handlers to the service router.
func registerRaceReplayStreamRoutes(mux *http.ServeMux, controller *RaceReplayStreamController) {
	mux.Handle("GET /sessions/{sessionId}/race/replay", bootstrap.ExtendDeadlines(http.HandlerFunc(controller.GetReplay), 0, replayWriteTimeout))
}
