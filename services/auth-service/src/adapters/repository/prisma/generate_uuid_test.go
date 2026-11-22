/**
##
## OverDrive 2026
## All Technical rights reserved
##
## generate_uuid_test.go - Unit tests for the user ID generator.
##
*/

package prismaadapter

import (
	"regexp"
	"testing"
)

// uuidV4Pattern is the canonical lower-case RFC 4122 form: version nibble 4, variant 10xx.
var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// TestGenerateUUID_Format proves generateUUID returns a canonical version 4 UUID. The User ID
// column is filled by this function, not by Postgres, for OAuth sign-ups, so the format must
// match what Prisma's uuid() default produces for password sign-ups.
func TestGenerateUUID_Format(t *testing.T) {
	for i := 0; i < 200; i++ {
		id, err := generateUUID()
		if err != nil {
			t.Fatalf("generateUUID: %v", err)
		}
		if !uuidV4Pattern.MatchString(id) {
			t.Fatalf("generateUUID() = %q, not a canonical v4 UUID", id)
		}
	}
}

// TestGenerateUUID_Distinct proves repeated calls do not collide over a reasonable sample.
func TestGenerateUUID_Distinct(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, err := generateUUID()
		if err != nil {
			t.Fatalf("generateUUID: %v", err)
		}
		if seen[id] {
			t.Fatalf("collision after %d IDs: %s", i, id)
		}
		seen[id] = true
	}
}
