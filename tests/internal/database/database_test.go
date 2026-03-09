/**
##
## OverDrive 2026
## All Technical rights reserved
##
## database_test.go - Unit tests for DB state helpers without a live database.
##
*/

package database_test

import (
	"testing"

	"overdrive/internal/database"
)

// TestIsConnectedWithoutClient verifies nil Prisma client state is reported as disconnected.
func TestIsConnectedWithoutClient(t *testing.T) {
	database.PrismaClient = nil

	if database.IsConnected() {
		t.Fatal("expected disconnected state when prisma client is nil")
	}
}
