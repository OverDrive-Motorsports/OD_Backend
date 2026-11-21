/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session.controller.go - HTTP handlers for the caller's own refresh sessions.
##
*/

package httpadapter

import (
	"errors"
	"net/http"
	"overdrive/services/auth-service/src/core/domain"
	"overdrive/services/auth-service/src/core/ports"
	"overdrive/shared/apierror"

	"github.com/steebchen/prisma-client-go/runtime/types"
)

type SessionController struct {
	usecase ports.SessionQueryUseCase
}

// NewSessionController builds and returns a session controller with its required dependencies.
func NewSessionController(usecase ports.SessionQueryUseCase) *SessionController {
	return &SessionController{usecase: usecase}
}

// sessionView is the public shape of a session. The refresh token, and its hash, never leave
// the service.
type sessionView struct {
	ID        string         `json:"id"`
	CreatedAt types.DateTime `json:"createdAt"`
	ExpiresAt types.DateTime `json:"expiresAt"`
}

func toSessionView(session domain.AuthSession) sessionView {
	return sessionView{ID: session.ID, CreatedAt: session.CreatedAt, ExpiresAt: session.ExpiresAt}
}

// ListSessions returns the caller's sessions as a bare array (empty, never null).
func (c *SessionController) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := c.usecase.GetUserSessions(r.Context(), userIDFrom(r))
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load user sessions", err))
		return
	}
	views := make([]sessionView, 0, len(sessions))
	for _, session := range sessions {
		views = append(views, toSessionView(session))
	}
	writeJSON(w, http.StatusOK, views)
}

// GetSession returns one of the caller's sessions; another user's session is not found.
func (c *SessionController) GetSession(w http.ResponseWriter, r *http.Request) {
	session, err := c.usecase.GetUserSession(r.Context(), userIDFrom(r), r.PathValue("sessionId"))
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load user session", err))
		return
	}
	if session == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, toSessionView(*session))
}

// DeleteSession revokes one of the caller's sessions (its refresh token stops working); another
// user's session is not found and stays untouched.
func (c *SessionController) DeleteSession(w http.ResponseWriter, r *http.Request) {
	err := c.usecase.RemoveUserSession(r.Context(), userIDFrom(r), r.PathValue("sessionId"))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, domain.ErrSessionNotFound):
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", err))
	default:
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to remove user session", err))
	}
}
