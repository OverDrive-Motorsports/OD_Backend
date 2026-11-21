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
	header string
}

func (f *fakeTokenValidator) ValidateBearerHeader(header string) bool {
	f.header = header
	return f.result
}

type fakeUserTokenValidator struct {
	role      domain.UserRole
	err       error
	gotHeader string
}

func (f *fakeUserTokenValidator) ValidateUserBearerHeader(header string) (domain.UserRole, error) {
	f.gotHeader = header
	return f.role, f.err
}

// TestAuthorizeUserRequestUseCase_Execute proves the header reaches the validator unchanged, the
// rejection reason is returned as-is (the HTTP layer maps it to a 401 code), any role is
// accepted, and a missing validator fails closed.
func TestAuthorizeUserRequestUseCase_Execute(t *testing.T) {
	validator := &fakeUserTokenValidator{err: domain.ErrUserTokenExpired}
	if err := NewAuthorizeUserRequestUseCase(validator).Execute("Bearer x"); !errors.Is(err, domain.ErrUserTokenExpired) || validator.gotHeader != "Bearer x" {
		t.Fatalf("expected the validator's reason and header, got err=%v header=%q", err, validator.gotHeader)
	}
	for _, role := range []domain.UserRole{domain.RoleUser, domain.RoleAdmin} {
		if err := NewAuthorizeUserRequestUseCase(&fakeUserTokenValidator{role: role}).Execute("Bearer ok"); err != nil {
			t.Fatalf("expected nil for a valid %s token, got %v", role, err)
		}
	}
	if err := NewAuthorizeUserRequestUseCase(nil).Execute("Bearer ok"); !errors.Is(err, domain.ErrInvalidUserToken) {
		t.Fatalf("expected ErrInvalidUserToken without a validator, got %v", err)
	}
}

// TestAuthorizeAdminRequestUseCase_Execute proves the admin routes accept the gateway token
// (without asking the user validator) and an admin token, refuse a valid user token with
// ErrForbiddenRole, pass any other rejection reason through, and fail closed without a validator.
func TestAuthorizeAdminRequestUseCase_Execute(t *testing.T) {
	gateway := &fakeTokenValidator{result: true}
	users := &fakeUserTokenValidator{err: domain.ErrInvalidUserToken}
	if err := NewAuthorizeAdminRequestUseCase(gateway, users).Execute("Bearer static"); err != nil || gateway.header != "Bearer static" || users.gotHeader != "" {
		t.Fatalf("gateway token: err=%v gateway header=%q user header=%q", err, gateway.header, users.gotHeader)
	}

	rejectStatic := &fakeTokenValidator{result: false}
	for name, tc := range map[string]struct {
		user *fakeUserTokenValidator
		want error
	}{
		"admin token":   {&fakeUserTokenValidator{role: domain.RoleAdmin}, nil},
		"user token":    {&fakeUserTokenValidator{role: domain.RoleUser}, domain.ErrForbiddenRole},
		"expired token": {&fakeUserTokenValidator{err: domain.ErrUserTokenExpired}, domain.ErrUserTokenExpired},
		"no token":      {&fakeUserTokenValidator{err: domain.ErrMissingUserToken}, domain.ErrMissingUserToken},
	} {
		err := NewAuthorizeAdminRequestUseCase(rejectStatic, tc.user).Execute("Bearer jwt")
		if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
			t.Fatalf("%s: got %v, want %v", name, err, tc.want)
		}
	}

	if err := NewAuthorizeAdminRequestUseCase(rejectStatic, nil).Execute("Bearer jwt"); !errors.Is(err, domain.ErrInvalidUserToken) {
		t.Fatalf("no user validator: expected ErrInvalidUserToken, got %v", err)
	}
}
