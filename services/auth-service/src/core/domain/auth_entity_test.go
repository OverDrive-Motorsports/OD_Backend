/**
##
## OverDrive 2026
## All Technical rights reserved
##
## auth_entity_test.go - Unit tests for the auth domain entities.
##
*/

package domain

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestUser_PasswordHashNeverSerialised proves a User marshalled to JSON - by a future endpoint
// or a log line - never contains its password hash, neither the field nor the value.
func TestUser_PasswordHashNeverSerialised(t *testing.T) {
	const hash = "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012"
	raw, err := json.Marshal(User{ID: "user-1", Email: "a@a.fr", PasswordHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if strings.Contains(body, hash) || strings.Contains(strings.ToLower(body), "password") {
		t.Fatalf("password hash leaked into JSON: %s", body)
	}
	if !strings.Contains(body, `"id":"user-1"`) {
		t.Fatalf("expected the other fields to be serialised: %s", body)
	}
}
