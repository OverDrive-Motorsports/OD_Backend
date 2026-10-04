/**
##
## OverDrive 2026
## All Technical rights reserved
##
## driver_feed.controller.go - HTTP handlers for a driver's onboard camera feed list.
##
*/

package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"

	"overdrive/services/race-data-service/src/core/domain"
	"overdrive/services/race-data-service/src/core/ports"
	"overdrive/shared/apierror"
)

type DriverFeedController struct {
	usecase ports.DriverFeedUseCase
}

// NewDriverFeedController builds and returns a driver feed controller with its required dependencies.
func NewDriverFeedController(usecase ports.DriverFeedUseCase) *DriverFeedController {
	return &DriverFeedController{usecase: usecase}
}

// GetDriverBroadcast returns `{sessionId, driverNumber, feeds}`; feeds is always an array.
func (c *DriverFeedController) GetDriverBroadcast(w http.ResponseWriter, r *http.Request) {
	driverNumber, ok := parsePathPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	payload, err := c.usecase.GetDriverFeeds(r.Context(), r.PathValue("sessionId"), driverNumber)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyDriverFeedErr(err, "failed to load driver feeds"))
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// PutDriverBroadcast replaces the driver's feed list from a `{"feeds": [...]}` body and returns
// the updated payload. Malformed body / invalid list -> 400, unknown session or driver -> 404.
func (c *DriverFeedController) PutDriverBroadcast(w http.ResponseWriter, r *http.Request) {
	driverNumber, ok := parsePathPositiveInt(w, r, "driverNumber")
	if !ok {
		return
	}
	feeds, ok := decodeFeedListBody(w, r)
	if !ok {
		return
	}
	payload, err := c.usecase.ReplaceDriverFeeds(r.Context(), r.PathValue("sessionId"), driverNumber, feeds)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyDriverFeedErr(err, "failed to store driver feeds"))
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// decodeFeedListBody strictly decodes a feed list body (unknown fields rejected, exactly one
// JSON value, "feeds" key mandatory). It writes the 400 itself and reports ok=false on failure.
func decodeFeedListBody(w http.ResponseWriter, r *http.Request) ([]domain.Feed, bool) {
	var body struct {
		Feeds *[]domain.Feed `json:"feeds"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid feed list body", err))
		return nil, false
	}
	if decoder.More() {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid feed list body", errors.New("trailing data after JSON body")))
		return nil, false
	}
	if body.Feeds == nil {
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid feed list body", errors.New("missing feeds field")))
		return nil, false
	}
	return *body.Feeds, true
}

// classifyDriverFeedErr maps the driver feed usecase sentinels to apierrors: invalid list -> 400
// (the domain message names index + field and is safe to surface), unknown session/driver ->
// 404, championship-service failure -> 502, anything else -> 500 with a fixed message.
func classifyDriverFeedErr(err error, internalMessage string) *apierror.Error {
	switch {
	case errors.Is(err, domain.ErrInvalidFeed):
		return apierror.Validation(err.Error(), err)
	case errors.Is(err, domain.ErrSessionNotFound):
		return apierror.NotFound("SESSION", "session not found", err)
	case errors.Is(err, domain.ErrDriverNotFound):
		return apierror.NotFound("DRIVER", "driver not found", err)
	case errors.Is(err, domain.ErrChampionshipUnavailable):
		return apierror.UpstreamUnavailable("failed to reach championship-service", err)
	default:
		return apierror.Internal(internalMessage, err)
	}
}
