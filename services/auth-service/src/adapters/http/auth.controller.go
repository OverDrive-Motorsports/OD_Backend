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
	"errors"
	"net/http"
	"overdrive/services/auth-service/src/core/domain"
	"overdrive/services/auth-service/src/core/ports"
	"overdrive/shared/apierror"
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
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid request body", err))
		return
	}
	session, err := c.usecase.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyLoginErr(err))
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
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid request body", err))
		return
	}

	session, err := c.usecase.Register(r.Context(), body.Email, body.Password, body.Username)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyRegisterErr(err))
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
		apierror.Write(w, r.URL.Path, apierror.Validation("invalid request body", err))
		return
	}

	session, err := c.usecase.Refresh(r.Context(), body.RefreshToken, body.SessionID)
	if err != nil {
		apierror.Write(w, r.URL.Path, classifyRefreshErr(err))
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// classifyLoginErr maps an AuthQueryUseCase.Login error into the matching apierror.
func classifyLoginErr(err error) *apierror.Error {
	if errors.Is(err, domain.ErrInvalidCredentials) {
		return apierror.Unauthorized("invalid credentials", nil)
	}
	return apierror.Internal("failed to process login", err)
}

// classifyRegisterErr maps an AuthQueryUseCase.Register error into the matching apierror.
func classifyRegisterErr(err error) *apierror.Error {
	if errors.Is(err, domain.ErrEmailAlreadyRegistered) {
		return apierror.AlreadyExists("EMAIL", "email already registered", nil)
	}
	return apierror.Internal("failed to process registration", err)
}

// classifyRefreshErr maps an AuthQueryUseCase.Refresh error into the matching apierror.
func classifyRefreshErr(err error) *apierror.Error {
	if errors.Is(err, domain.ErrInvalidSession) {
		return apierror.Unauthorized("invalid session", nil)
	}
	if errors.Is(err, domain.ErrInvalidToken) {
		return apierror.Unauthorized("invalid token", nil)
	}
	return apierror.Internal("failed to process refresh", err)
}
