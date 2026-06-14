/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## auth.controller.go - Minimal mock auth endpoints for local development.
	##
	*/
package httpadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type AuthController struct {
	mu       sync.Mutex
	users    map[string]string // email -> passwordHash
	sessions map[string]string // refreshToken -> email
}

func NewAuthController() *AuthController {
	rand.Seed(time.Now().UnixNano())
	return &AuthController{
		users:    make(map[string]string),
		sessions: make(map[string]string),
	}
}

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (c *AuthController) Signup(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[auth] Signup start from %s\n", r.RemoteAddr)
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		fmt.Printf("[auth] Signup method not allowed from %s\n", r.RemoteAddr)
		return
	}

	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("[auth] Signup decode error from %s: %v\n", r.RemoteAddr, err)
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	fmt.Printf("[auth] Signup decoded from %s: %+v\n", r.RemoteAddr, req)

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := req.Password
	if email == "" || password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}
	if len(password) < 6 {
		http.Error(w, "password too short", http.StatusBadRequest)
		return
	}

	pwHash := hashPassword(password)
	fmt.Printf("[auth] Signup hashed password for %s\n", email)

	c.mu.Lock()
	if _, exists := c.users[email]; exists {
		c.mu.Unlock()
		fmt.Printf("[auth] Signup user already exists: %s\n", email)
		http.Error(w, "user already exists", http.StatusConflict)
		return
	}

	c.users[email] = pwHash
	c.mu.Unlock()
	fmt.Printf("[auth] Signup stored user: %s\n", email)

	tokens := c.generateTokens(email)
	fmt.Printf("[auth] Signup tokens generated for %s\n", email)
	writeJSON(w, http.StatusOK, tokens)
	fmt.Printf("[auth] Signup response written for %s\n", email)
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[auth] Login start from %s\n", r.RemoteAddr)
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		fmt.Printf("[auth] Login method not allowed from %s\n", r.RemoteAddr)
		return
	}

	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("[auth] Login decode error from %s: %v\n", r.RemoteAddr, err)
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	fmt.Printf("[auth] Login decoded from %s: %+v\n", r.RemoteAddr, req)

	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := req.Password
	if email == "" || password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	stored, ok := c.users[email]
	c.mu.Unlock()
	if !ok {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if !verifyPassword(stored, password) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	tokens := c.generateTokens(email)
	writeJSON(w, http.StatusOK, tokens)
}

func (c *AuthController) Refresh(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("[auth] Refresh start from %s\n", r.RemoteAddr)
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		fmt.Printf("[auth] Refresh method not allowed from %s\n", r.RemoteAddr)
		return
	}

	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Printf("[auth] Refresh decode error from %s: %v\n", r.RemoteAddr, err)
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	fmt.Printf("[auth] Refresh decoded from %s: %+v\n", r.RemoteAddr, req)

	token := strings.TrimSpace(req.RefreshToken)
	if token == "" {
		http.Error(w, "refresh_token is required", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	email, ok := c.sessions[token]
	// optional rotation: delete old refresh token
	if ok {
		delete(c.sessions, token)
	}
	c.mu.Unlock()

	if !ok {
		http.Error(w, "invalid refresh token", http.StatusUnauthorized)
		return
	}

	tokens := c.generateTokens(email)
	writeJSON(w, http.StatusOK, tokens)
}

func (c *AuthController) generateTokens(email string) map[string]interface{} {
	access := fmt.Sprintf("mock-access-%d-%d", time.Now().UnixNano(), rand.Intn(100000))
	refresh := fmt.Sprintf("mock-refresh-%d-%d", time.Now().UnixNano(), rand.Intn(100000))
	expiresIn := 3600

	c.mu.Lock()
	c.sessions[refresh] = email
	c.mu.Unlock()

	return map[string]interface{}{
		"access_token":  access,
		"refresh_token": refresh,
		"expires_in":    expiresIn,
	}
}

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

func verifyPassword(storedHash, password string) bool {
	return storedHash == hashPassword(password)
}
