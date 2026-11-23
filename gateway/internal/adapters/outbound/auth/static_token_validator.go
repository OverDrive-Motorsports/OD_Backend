/**
##
## OverDrive 2026
## All Technical rights reserved
##
## static_token_validator.go - Outbound adapter that validates a static bearer token.
##
*/

// Package auth contains outbound auth adapter implementations.

package auth

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type StaticTokenValidator struct {
	expectedToken string
}

func NewStaticTokenValidator(expectedToken string) StaticTokenValidator {
	return StaticTokenValidator{expectedToken: expectedToken}
}

func (v StaticTokenValidator) ValidateBearerHeader(header string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return false
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return false
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return false
	}

	return token == v.expectedToken
}

type JWTValidator struct {
	jwtSecret string
}

func NewJWTValidator(jwtSecret string) JWTValidator {
	return JWTValidator{jwtSecret: jwtSecret}
}

func (v JWTValidator) ValidateBearerHeader(header string) bool {
	header = strings.TrimSpace(header)
	if header == "" || v.jwtSecret == "" {
		return false
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return false
	}

	if !strings.EqualFold(parts[0], "Bearer") {
		return false
	}

	tokenString := strings.TrimSpace(parts[1])
	if tokenString == "" {
		return false
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(v.jwtSecret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid {
		return false
	}

	return true
}
