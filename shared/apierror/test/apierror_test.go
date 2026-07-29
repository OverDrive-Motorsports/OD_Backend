/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## apierror_test.go - Test file for shared/apierror.
	##
*/

package test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"overdrive/shared/apierror"
)

// captureStderr redirects os.Stderr for the duration of fn and returns everything written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	os.Stderr = w

	fn()

	_ = w.Close()
	os.Stderr = original

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll() failed: %v", err)
	}
	return string(out)
}

func TestConstructors(t *testing.T) {
	cases := []struct {
		name       string
		build      func() *apierror.Error
		wantCode   apierror.Code
		wantStatus apierror.Status
	}{
		{"Validation", func() *apierror.Error { return apierror.Validation("bad input", nil) }, apierror.CodeValidationError, apierror.StatusBadRequest},
		{"InvalidField", func() *apierror.Error { return apierror.InvalidField("email", "invalid email", nil) }, "INVALID_EMAIL", apierror.StatusBadRequest},
		{"Unauthorized", func() *apierror.Error { return apierror.Unauthorized("no auth", nil) }, apierror.CodeUnauthorized, apierror.StatusUnauthorized},
		{"NotFound", func() *apierror.Error { return apierror.NotFound("session", "not found", nil) }, "SESSION_NOT_FOUND", apierror.StatusNotFound},
		{"Conflict", func() *apierror.Error { return apierror.Conflict("user", "conflict", nil) }, "USER_CONFLICT", apierror.StatusConflict},
		{"Internal", func() *apierror.Error { return apierror.Internal("oops", nil) }, apierror.CodeInternalError, apierror.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := tc.build()
			if e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", e.Code, tc.wantCode)
			}
			if e.Status != tc.wantStatus {
				t.Errorf("Status = %d, want %d", e.Status, tc.wantStatus)
			}
		})
	}
}

func TestError_ErrorAndUnwrap(t *testing.T) {
	wrapped := errors.New("technical detail")
	e := apierror.New(apierror.CodeInternalError, apierror.StatusInternalServerError, "client message", wrapped)

	if got := e.Error(); got != "client message" {
		t.Errorf("Error() = %q, want %q", got, "client message")
	}
	if !errors.Is(e, wrapped) {
		t.Errorf("errors.Is(e, wrapped) = false, want true via Unwrap()")
	}
}

func TestError_LogDebug(t *testing.T) {
	e := apierror.NotFound("SESSION", "No session found for this ID.", errors.New("row not found"))

	output := captureStderr(t, func() {
		e.LogDebug("/sessions/42")
	})

	want := "status=404 code=SESSION_NOT_FOUND path=/sessions/42 err=row not found\n"
	if output != want {
		t.Errorf("LogDebug output = %q, want %q", output, want)
	}
}

func TestWrite(t *testing.T) {
	e := apierror.NotFound("SESSION", "No session found for this ID.", errors.New("row not found"))
	rec := httptest.NewRecorder()

	_ = captureStderr(t, func() {
		apierror.Write(rec, "/sessions/42", e)
	})

	res := rec.Result()
	if res.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Status  int    `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Error.Code != "SESSION_NOT_FOUND" {
		t.Errorf("body.Error.Code = %q, want %q", body.Error.Code, "SESSION_NOT_FOUND")
	}
	if body.Error.Status != 404 {
		t.Errorf("body.Error.Status = %d, want 404", body.Error.Status)
	}
	if body.Error.Message != "No session found for this ID." {
		t.Errorf("body.Error.Message = %q, want %q", body.Error.Message, "No session found for this ID.")
	}
}
