/**
 ##
 ## OverDrive 2026
 ## All Technical rights reserved
 ##
 ## authorize_request_test.go - Unit tests for AuthorizeRequestUseCase.
 ##
 */

package usecases

import "testing"

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
