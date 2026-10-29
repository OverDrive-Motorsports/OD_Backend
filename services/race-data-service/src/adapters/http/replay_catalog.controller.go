/**
##
## OverDrive 2026
## All Technical rights reserved
##
## replay_catalog.controller.go - HTTP handler for GET /championships/{code}/replays.
##
*/

package httpadapter

import (
	"errors"
	"net/http"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
	"overdrive/shared/apierror"
)

type ReplayCatalogController struct {
	usecase ports.ReplayCatalogUseCase
}

// NewReplayCatalogController builds and returns a replay catalog controller with its required dependencies.
func NewReplayCatalogController(usecase ports.ReplayCatalogUseCase) *ReplayCatalogController {
	return &ReplayCatalogController{usecase: usecase}
}

// ListReplays answers the championship's playable replays as a bare JSON array, newest first:
// `[]` when there is none, 404 CHAMPIONSHIP_NOT_FOUND for an unknown code, 502 when
// championship-service is unreachable, 500 on a local store failure.
func (c *ReplayCatalogController) ListReplays(w http.ResponseWriter, r *http.Request) {
	entries, err := c.usecase.ListReplays(r.Context(), r.PathValue("code"))
	if err != nil {
		if errors.Is(err, domain.ErrChampionshipUnavailable) {
			apierror.Write(w, r.URL.Path, apierror.UpstreamUnavailable("failed to reach championship-service", err))
			return
		}
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load replays", err))
		return
	}
	if entries == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("CHAMPIONSHIP", "championship not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
