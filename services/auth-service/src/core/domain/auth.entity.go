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

type User struct {
	ID           string         `json:"id"`
	Email        string         `json:"email"`
	DisplayName  string         `json:"displayName"`
	PasswordHash string         `json:"passwordHash"`
	CreatedAt    types.DateTime `json:"createdAt"`
	UpdatedAt    types.DateTime `json:"updatedAt"`
}

type AuthSession struct {
	ID               string         `json:"id"`
	UserID           string         `json:"userID"`
	RefreshTokenHash string         `json:"refreshTokenHash"`
	ExpiresAt        types.DateTime `json:"expiresAt"`
	CreatedAt        types.DateTime `json:"createdAt"`
}
