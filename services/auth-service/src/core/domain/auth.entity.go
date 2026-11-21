/**
##
## OverDrive 2026
## All Technical rights reserved
##
## catalog.service.go - Package ports source file for services/championship-service/src/core/ports.
##
*/

package domain

import "github.com/steebchen/prisma-client-go/runtime/types"

// UserRole is the access level of an account, issued in the `role` claim of its access tokens.
// Values match the UserRole enum of resources/schema.prisma.
type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

type User struct {
	ID           string         `json:"id"`
	Email        string         `json:"email"`
	DisplayName  string         `json:"displayName"`
	PasswordHash string         `json:"-"` // never serialised, even by accident
	UsesOAuth    bool           `json:"usesOAuth"`
	Role         UserRole       `json:"role"`
	CreatedAt    types.DateTime `json:"createdAt"`
	UpdatedAt    types.DateTime `json:"updatedAt"`
}

type AuthSession struct {
	ID           string         `json:"id"`
	UserID       string         `json:"userID"`
	RefreshToken string         `json:"refreshToken"`
	ExpiresAt    types.DateTime `json:"expiresAt"`
	CreatedAt    types.DateTime `json:"createdAt"`
}

type LoginResponse struct {
	Token        string         `json:"token"`
	SessionID    string         `json:"sessionId"`
	RefreshToken string         `json:"refreshToken"`
	ExpiresAt    types.DateTime `json:"expiresAt"`
}

type RefreshResponse struct {
	Token        string         `json:"token"`
	RefreshToken string         `json:"refreshToken"`
	ExpiresAt    types.DateTime `json:"expiresAt"`
}
