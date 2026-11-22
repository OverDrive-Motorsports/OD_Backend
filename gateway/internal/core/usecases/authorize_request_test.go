/**
##
## OverDrive 2026
## All Technical rights reserved
##
## authorize_request_test.go - Unit tests for AuthorizeRequestUseCase.
##
*/

package usecases

import (
	"errors"
	"testing"

	"overdrive/gateway/internal/core/domain"
)

type fakeTokenValidator struct {
	result bool
	called bool
	header string
}

func (f *fakeTokenValidator) ValidateBearerHeader(header string) bool {
	f.called = true
	f.header = header
	return f.result
}

func TestAuthorizeRequestUseCase_DelegatesToValidator(t *testing.T) {
	validator := &fakeTokenValidator{result: true}
	useCase := NewAuthorizeRequestUseCase(validator)

	ok := useCase.Execute("Bearer some-token")

	if !ok {
		t.Fatal("expected Execute to return true when validator approves")
	}
	if !validator.called {
		t.Fatal("expected Execute to call the underlying validator")
	}
	if validator.header != "Bearer some-token" {
		t.Fatalf("expected header to be forwarded unchanged, got %q", validator.header)
	}
}

func TestAuthorizeRequestUseCase_RejectsWhenValidatorRejects(t *testing.T) {
	validator := &fakeTokenValidator{result: false}
	useCase := NewAuthorizeRequestUseCase(validator)

	if useCase.Execute("Bearer bad-token") {
		t.Fatal("expected Execute to return false when validator rejects")
	}
}

type fakeUserTokenValidator struct {
	err       error
	gotHeader string
}

func (f *fakeUserTokenValidator) ValidateUserBearerHeader(header string) error {
	f.gotHeader = header
	return f.err
}

// TestAuthorizeUserRequestUseCase_Execute proves the header reaches the validator unchanged and
// the rejection reason is returned as-is (the HTTP layer maps it to a 401 code).
func TestAuthorizeUserRequestUseCase_Execute(t *testing.T) {
	validator := &fakeUserTokenValidator{err: domain.ErrUserTokenExpired}
	if err := NewAuthorizeUserRequestUseCase(validator).Execute("Bearer x"); !errors.Is(err, domain.ErrUserTokenExpired) || validator.gotHeader != "Bearer x" {
		t.Fatalf("expected the validator's reason and header, got err=%v header=%q", err, validator.gotHeader)
	}
	if err := NewAuthorizeUserRequestUseCase(&fakeUserTokenValidator{}).Execute("Bearer ok"); err != nil {
		t.Fatalf("expected nil for a valid token, got %v", err)
	}
}
