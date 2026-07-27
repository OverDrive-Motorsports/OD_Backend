/**
##
## OverDrive 2026
## All Technical rights reserved
##
## session.service.go - Package ports source file for services/auth-service/src/core/ports.
##
*/

package httpadapter

import (
	"encoding/json"
	"net/http"
	"overdrive/services/auth-service/src/core/ports"
)

type AuthController struct {
	usecase ports.AuthQueryUseCase
}

func NewAuthController(usecase ports.AuthQueryUseCase) *AuthController {
	return &AuthController{usecase: usecase}
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login Allow the user to login with its credentials. Returns the created session
func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	session, err := c.usecase.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

type registerBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

// Register Allow the user to register a new account. Returns the created session
func (c *AuthController) Register(w http.ResponseWriter, r *http.Request) {
	var body registerBody

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	session, err := c.usecase.Register(r.Context(), body.Email, body.Password, body.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

type refreshBody struct {
	SessionID    string `json:"sessionId"`
	RefreshToken string `json:"refreshToken"`
}

// Refresh Allow the user to register a new account. Returns the created session
func (c *AuthController) Refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshBody

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	session, err := c.usecase.Refresh(r.Context(), body.RefreshToken, body.SessionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, session)
}
