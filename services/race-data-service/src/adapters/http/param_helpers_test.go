/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## param_helpers_test.go - Package httpadapter source file for services/race-data-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestParseOptionalPositiveInt covers: absent parameter (nil, ok=true), a valid positive integer,
// a non-numeric value (400), and zero/negative values (both rejected as "not positive").
func TestParseOptionalPositiveInt(t *testing.T) {
	t.Run("absent parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rec := httptest.NewRecorder()
		got, ok := parseOptionalPositiveInt(rec, req, "driverNumber")
		if !ok || got != nil {
			t.Fatalf("expected nil, true for an absent parameter, got %v %v", got, ok)
		}
	})

	t.Run("valid positive integer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x?driverNumber=44", nil)
		rec := httptest.NewRecorder()
		got, ok := parseOptionalPositiveInt(rec, req, "driverNumber")
		if !ok || got == nil || *got != 44 {
			t.Fatalf("expected 44, true, got %v %v", got, ok)
		}
	})

	t.Run("non-numeric value -> 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x?driverNumber=abc", nil)
		rec := httptest.NewRecorder()
		_, ok := parseOptionalPositiveInt(rec, req, "driverNumber")
		if ok {
			t.Fatal("expected ok=false for a non-numeric value")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("zero is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x?driverNumber=0", nil)
		rec := httptest.NewRecorder()
		_, ok := parseOptionalPositiveInt(rec, req, "driverNumber")
		if ok {
			t.Fatal("expected ok=false for zero")
		}
	})

	t.Run("negative is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x?driverNumber=-1", nil)
		rec := httptest.NewRecorder()
		_, ok := parseOptionalPositiveInt(rec, req, "driverNumber")
		if ok {
			t.Fatal("expected ok=false for a negative value")
		}
	})
}

// TestParsePathPositiveInt covers a valid path value, a missing/empty path value (400), and a
// non-positive value (400).
func TestParsePathPositiveInt(t *testing.T) {
	t.Run("valid positive integer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.SetPathValue("driverNumber", "44")
		rec := httptest.NewRecorder()
		got, ok := parsePathPositiveInt(rec, req, "driverNumber")
		if !ok || got != 44 {
			t.Fatalf("expected 44, true, got %v %v", got, ok)
		}
	})

	t.Run("missing path value -> 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		rec := httptest.NewRecorder()
		_, ok := parsePathPositiveInt(rec, req, "driverNumber")
		if ok {
			t.Fatal("expected ok=false for a missing path value")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("zero is rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.SetPathValue("driverNumber", "0")
		rec := httptest.NewRecorder()
		_, ok := parsePathPositiveInt(rec, req, "driverNumber")
		if ok {
			t.Fatal("expected ok=false for zero")
		}
	})
}
