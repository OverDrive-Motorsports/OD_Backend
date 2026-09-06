package httpadapter

import (
	"net/http"
	"overdrive/services/auth-service/src/core/ports"
	"overdrive/shared/apierror"
)

type SessionController struct {
	usecase ports.SessionQueryUseCase
}

// NewSessionController builds and returns a session controller with its required dependencies.
func NewSessionController(usecase ports.SessionQueryUseCase) *SessionController {
	return &SessionController{usecase: usecase}
}

// GetUserSessions returns all auth sessions for a specific user
func (c *SessionController) GetUserSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	sessions, err := c.usecase.GetUserSessions(r.Context(), userID)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load user sessions", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(sessions), "sessions": sessions})
}

// GetUserSession returns a specific session for one user
func (c *SessionController) GetUserSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	userID := r.PathValue("userID")
	session, err := c.usecase.GetUserSession(r.Context(), userID, sessionID)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to load user session", err))
		return
	}
	if session == nil {
		apierror.Write(w, r.URL.Path, apierror.NotFound("SESSION", "session not found", nil))
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (c *SessionController) AddUserSession(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	session, err := c.usecase.AddUserSession(r.Context(), userID)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to create user session", err))
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (c *SessionController) RemoveUserSession(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	sessionID := r.PathValue("sessionID")
	err := c.usecase.RemoveUserSession(r.Context(), userID, sessionID)
	if err != nil {
		apierror.Write(w, r.URL.Path, apierror.Internal("failed to remove user session", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}
