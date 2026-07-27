package httpadapter

import (
	"net/http"
	"overdrive/services/auth-service/src/core/ports"
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (c *SessionController) AddUserSession(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	session, err := c.usecase.AddUserSession(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (c *SessionController) RemoveUserSession(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	sessionID := r.PathValue("sessionID")
	err := c.usecase.RemoveUserSession(r.Context(), userID, sessionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}
