/**
	##
	## OverDrive 2026
	## All Technical rights reserved
	##
	## auth_controller_test.go - Package httpadapter source file for services/auth-service/src/adapters/http.
	##
*/

package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/auth-service/src/core/domain"
)

// fakeAuthUseCase implements ports.AuthQueryUseCase for AuthController tests. Each method
// returns whatever canned result/error is configured, so every classifyXErr branch can be
// exercised directly.
type fakeAuthUseCase struct {
	loginResp *domain.LoginResponse
	loginErr  error

	registerResp *domain.LoginResponse
	registerErr  error

	refreshResp *domain.RefreshResponse
	refreshErr  error
}

func (f *fakeAuthUseCase) Login(ctx context.Context, email string, password string) (*domain.LoginResponse, error) {
	return f.loginResp, f.loginErr
}

func (f *fakeAuthUseCase) Register(ctx context.Context, email string, password string, userName string) (*domain.LoginResponse, error) {
	return f.registerResp, f.registerErr
}

func (f *fakeAuthUseCase) Refresh(ctx context.Context, refreshToken string, sessionID string) (*domain.RefreshResponse, error) {
	return f.refreshResp, f.refreshErr
}

func (f *fakeAuthUseCase) VerifyToken(tokenString string) (string, error) {
	panic("not used by AuthController")
}

// errorEnvelope mirrors shared/apierror's client-facing JSON shape.
type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Status  int    `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeErrorEnvelope(t *testing.T, body []byte) errorEnvelope {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("failed to decode error envelope: %v (body: %s)", err, body)
	}
	return env
}

// TestAuthController_Login_Success proves a valid body reaches the usecase and its response is
// written back as-is with a 200.
func TestAuthController_Login_Success(t *testing.T) {
	usecase := &fakeAuthUseCase{loginResp: &domain.LoginResponse{Token: "t", SessionID: "s1"}}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"a@b.com","password":"pw"}`))
	rec := httptest.NewRecorder()
	controller.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// TestAuthController_Login_MalformedBody proves an undecodable JSON body is rejected with 400
// before the usecase is ever consulted.
func TestAuthController_Login_MalformedBody(t *testing.T) {
	usecase := &fakeAuthUseCase{loginErr: errors.New("must not be called")}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()
	controller.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed body, got %d", rec.Code)
	}
}

// TestAuthController_Login_InvalidCredentials proves ErrInvalidCredentials is classified as
// 401, with the client-facing message never containing the raw sentinel error text (there is
// none to leak here, but this pins the mapping regardless).
func TestAuthController_Login_InvalidCredentials(t *testing.T) {
	usecase := &fakeAuthUseCase{loginErr: domain.ErrInvalidCredentials}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"a@b.com","password":"wrong"}`))
	rec := httptest.NewRecorder()
	controller.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	env := decodeErrorEnvelope(t, rec.Body.Bytes())
	if env.Error.Status != http.StatusUnauthorized {
		t.Fatalf("expected envelope status 401, got %d", env.Error.Status)
	}
}

// TestAuthController_Login_GenericFailure proves an unclassified repository-level failure is
// reported as 500, and that the fake usecase's real underlying error text never leaks into the
// client-facing response body.
func TestAuthController_Login_GenericFailure(t *testing.T) {
	technicalDetail := "pq: connection refused to postgres replica-7"
	usecase := &fakeAuthUseCase{loginErr: errors.New(technicalDetail)}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"a@b.com","password":"pw"}`))
	rec := httptest.NewRecorder()
	controller.Login(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), technicalDetail) {
		t.Fatalf("expected the technical error detail to never leak into the response body, got %s", rec.Body.String())
	}
}

// TestAuthController_Register_Success proves the normal-case path.
func TestAuthController_Register_Success(t *testing.T) {
	usecase := &fakeAuthUseCase{registerResp: &domain.LoginResponse{Token: "t"}}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"a@b.com","password":"pw","username":"a"}`))
	rec := httptest.NewRecorder()
	controller.Register(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// TestAuthController_Register_MalformedBody proves the same 400 guard applies to Register.
func TestAuthController_Register_MalformedBody(t *testing.T) {
	usecase := &fakeAuthUseCase{}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	controller.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestAuthController_Register_DuplicateEmail proves ErrEmailAlreadyRegistered is classified as
// 409.
func TestAuthController_Register_DuplicateEmail(t *testing.T) {
	usecase := &fakeAuthUseCase{registerErr: domain.ErrEmailAlreadyRegistered}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"a@b.com","password":"pw","username":"a"}`))
	rec := httptest.NewRecorder()
	controller.Register(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

// TestAuthController_Register_GenericFailure proves a generic error is a 500 without leaking
// its technical detail.
func TestAuthController_Register_GenericFailure(t *testing.T) {
	technicalDetail := "insert failed: unique constraint violated on shadow column"
	usecase := &fakeAuthUseCase{registerErr: errors.New(technicalDetail)}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"a@b.com","password":"pw","username":"a"}`))
	rec := httptest.NewRecorder()
	controller.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), technicalDetail) {
		t.Fatalf("expected the technical error detail to never leak into the response body, got %s", rec.Body.String())
	}
}

// TestAuthController_Refresh_Success proves the normal-case path.
func TestAuthController_Refresh_Success(t *testing.T) {
	usecase := &fakeAuthUseCase{refreshResp: &domain.RefreshResponse{Token: "t"}}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"sessionId":"s1","refreshToken":"rt"}`))
	rec := httptest.NewRecorder()
	controller.Refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

// TestAuthController_Refresh_MalformedBody proves the same 400 guard applies to Refresh.
func TestAuthController_Refresh_MalformedBody(t *testing.T) {
	usecase := &fakeAuthUseCase{}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	controller.Refresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestAuthController_Refresh_InvalidSession proves ErrInvalidSession maps to 401.
func TestAuthController_Refresh_InvalidSession(t *testing.T) {
	usecase := &fakeAuthUseCase{refreshErr: domain.ErrInvalidSession}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"sessionId":"missing","refreshToken":"rt"}`))
	rec := httptest.NewRecorder()
	controller.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// TestAuthController_Refresh_InvalidToken proves ErrInvalidToken also maps to 401 (both
// session-related failures share the same status, per classifyRefreshErr, even though they're
// distinguishable sentinels unlike Login's anti-enumeration case).
func TestAuthController_Refresh_InvalidToken(t *testing.T) {
	usecase := &fakeAuthUseCase{refreshErr: domain.ErrInvalidToken}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"sessionId":"s1","refreshToken":"wrong"}`))
	rec := httptest.NewRecorder()
	controller.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// TestAuthController_Refresh_GenericFailure proves a generic error is a 500 without leaking
// its technical detail.
func TestAuthController_Refresh_GenericFailure(t *testing.T) {
	technicalDetail := "context deadline exceeded talking to postgres"
	usecase := &fakeAuthUseCase{refreshErr: errors.New(technicalDetail)}
	controller := NewAuthController(usecase)

	req := httptest.NewRequest(http.MethodPost, "/refresh", strings.NewReader(`{"sessionId":"s1","refreshToken":"rt"}`))
	rec := httptest.NewRecorder()
	controller.Refresh(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), technicalDetail) {
		t.Fatalf("expected the technical error detail to never leak into the response body, got %s", rec.Body.String())
	}
}
