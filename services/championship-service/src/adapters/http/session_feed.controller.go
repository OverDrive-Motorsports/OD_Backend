/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session_feed.controller.go - HTTP handlers for a session's global video feed list.
##
*/

package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"

	"overdrive/services/championship-service/src/core/domain"
	"overdrive/services/championship-service/src/core/ports"
	"overdrive/shared/apierror"
)

// feedListRequest is the body of PUT /sessions/{sessionId}/broadcast.
type feedListRequest struct {
	Feeds []domain.Feed `json:"feeds"`
}

type SessionFeedController struct {
	usecase ports.SessionFeedUseCase
}

// NewSessionFeedController builds and returns a session feed controller with its required dependencies.
func NewSessionFeedController(usecase ports.SessionFeedUseCase) *SessionFeedController {
	return &SessionFeedController{usecase: usecase}
}

// GetSessionBroadcast returns `{sessionId, feeds}` for the session; feeds is always an array.
func (c *SessionFeedController) GetSessionBroadcast(w http.ResponseWriter, r *http.Request) {
	payload, err := c.usecase.GetSessionFeeds(r.Context(), r.PathValue("sessionId"))
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load session feeds", err))
		return
	}
	if payload == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

// PutSessionBroadcast replaces the whole feed list of the session from a `{"feeds": [...]}`
// body and returns the updated payload. A malformed body or a list rejected by
// domain.ValidateFeeds is a 400; an unknown session is a 404.
func (c *SessionFeedController) PutSessionBroadcast(w http.ResponseWriter, r *http.Request) {
	feeds, ok := decodeFeedListBody(w, r)
	if !ok {
		return
	}
	payload, err := c.usecase.ReplaceSessionFeeds(r.Context(), r.PathValue("sessionId"), feeds)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyFeedErr(err))
		return
	}
	if payload == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
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

// classifyFeedErr maps a feed usecase error: a domain.ErrInvalidFeed is a client error whose
// wrapped message (index + field) is safe to surface; anything else is an internal failure.
func classifyFeedErr(err error) *apierror.Error {
	if errors.Is(err, domain.ErrInvalidFeed) {
		return apierror.Validation(err.Error(), err)
	}
	return apierror.Internal("failed to store session feeds", err)
}
