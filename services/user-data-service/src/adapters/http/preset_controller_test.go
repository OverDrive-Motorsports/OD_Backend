/**
##
## OverDrive 2026
## All Technical rights reserved
##
## preset_controller_test.go - Tests the preset routes through the real router (auth included).
##
*/

package httpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"overdrive/services/user-data-service/src/core/domain"
	"overdrive/services/user-data-service/src/core/usecases"
	"overdrive/shared/authjwt"
)

// fakeVerifier maps tokens to user IDs: "alice-token" → alice, "expired" → ErrTokenExpired,
// anything else → ErrInvalidToken.
type fakeVerifier struct{}

func (fakeVerifier) Verify(token string) (string, error) {
	switch token {
	case "alice-token":
		return "alice", nil
	case "bob-token":
		return "bob", nil
	case "expired":
		return "", authjwt.ErrTokenExpired
	default:
		return "", authjwt.ErrInvalidToken
	}
}

// fakePresetUseCase records the user ID and inputs it receives and returns canned results.
type fakePresetUseCase struct {
	presets    []domain.Preset
	preset     *domain.Preset
	err        error
	gotUserID  string
	gotID      string
	gotNew     *domain.NewPreset
	gotChanges *domain.PresetChanges
}

func (f *fakePresetUseCase) List(_ context.Context, userID string) ([]domain.Preset, error) {
	f.gotUserID = userID
	return f.presets, f.err
}

func (f *fakePresetUseCase) Get(_ context.Context, userID, id string) (*domain.Preset, error) {
	f.gotUserID, f.gotID = userID, id
	return f.preset, f.err
}

func (f *fakePresetUseCase) Create(_ context.Context, userID string, input domain.NewPreset) (*domain.Preset, error) {
	f.gotUserID, f.gotNew = userID, &input
	return f.preset, f.err
}

func (f *fakePresetUseCase) Update(_ context.Context, userID, id string, changes domain.PresetChanges) (*domain.Preset, error) {
	f.gotUserID, f.gotID, f.gotChanges = userID, id, &changes
	return f.preset, f.err
}

func (f *fakePresetUseCase) Delete(_ context.Context, userID, id string) error {
	f.gotUserID, f.gotID = userID, id
	return f.err
}

func serve(usecase *fakePresetUseCase, method, path, token, body string) *httptest.ResponseRecorder {
	router := NewRouter(NewHealthController(usecases.NewGetHealthUseCase("user-data-service")), NewPresetController(usecase), fakeVerifier{})
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TestPresetRoutes_Authentication proves every preset route requires a bearer token (401
// UNAUTHORIZED without, TOKEN_EXPIRED / INVALID_TOKEN on a bad one) while /health stays open,
// and the usecase is never reached unauthenticated.
func TestPresetRoutes_Authentication(t *testing.T) {
	usecase := &fakePresetUseCase{}
	for _, route := range [][2]string{{"GET", "/"}, {"POST", "/"}, {"GET", "/p1"}, {"PATCH", "/p1"}, {"DELETE", "/p1"}} {
		for token, code := range map[string]string{"": "UNAUTHORIZED", "expired": "TOKEN_EXPIRED", "forged": "INVALID_TOKEN"} {
			rec := serve(usecase, route[0], route[1], token, `{}`)
			if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), code) {
				t.Fatalf("%s %s token=%q: status = %d body=%s", route[0], route[1], token, rec.Code, rec.Body.String())
			}
		}
	}
	if usecase.gotUserID != "" {
		t.Fatal("usecase reached without authentication")
	}
	if rec := serve(usecase, "GET", "/health", "", ""); rec.Code != http.StatusOK {
		t.Fatalf("/health must stay open, got %d", rec.Code)
	}
}

// TestPresetRoutes_NormalCases proves each route answers its success status, forwards the user
// ID taken from the token and the path ID, and passes the raw layout document through.
func TestPresetRoutes_NormalCases(t *testing.T) {
	preset := &domain.Preset{ID: "p1", Name: "Race view", IsDefault: true, Data: []byte(`{"widgets":[]}`)}

	usecase := &fakePresetUseCase{presets: []domain.Preset{*preset}}
	rec := serve(usecase, "GET", "/", "alice-token", "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), "[") || usecase.gotUserID != "alice" {
		t.Fatalf("list: status = %d user=%q body=%s", rec.Code, usecase.gotUserID, rec.Body.String())
	}
	for _, want := range []string{`"id":"p1"`, `"name":"Race view"`, `"isDefault":true`, `"data":{"widgets":[]}`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("list: expected %s in %s", want, rec.Body.String())
		}
	}

	usecase = &fakePresetUseCase{preset: preset}
	if rec := serve(usecase, "GET", "/p1", "bob-token", ""); rec.Code != http.StatusOK || usecase.gotUserID != "bob" || usecase.gotID != "p1" {
		t.Fatalf("get: status = %d user=%q id=%q", rec.Code, usecase.gotUserID, usecase.gotID)
	}

	usecase = &fakePresetUseCase{preset: preset}
	rec = serve(usecase, "POST", "/", "alice-token", `{"name":"Race view","isDefault":true,"data":{"widgets":[{"type":"video"}]}}`)
	if rec.Code != http.StatusCreated || usecase.gotNew == nil || usecase.gotNew.Name != "Race view" || !usecase.gotNew.IsDefault || string(usecase.gotNew.Data) != `{"widgets":[{"type":"video"}]}` {
		t.Fatalf("create: status = %d input=%+v", rec.Code, usecase.gotNew)
	}

	usecase = &fakePresetUseCase{preset: preset}
	rec = serve(usecase, "PATCH", "/p1", "alice-token", `{"isDefault":true}`)
	if rec.Code != http.StatusOK || usecase.gotChanges == nil || usecase.gotChanges.IsDefault == nil || !*usecase.gotChanges.IsDefault || usecase.gotChanges.Name != nil || usecase.gotChanges.Data != nil {
		t.Fatalf("update: status = %d changes=%+v", rec.Code, usecase.gotChanges)
	}

	usecase = &fakePresetUseCase{}
	if rec := serve(usecase, "DELETE", "/p1", "alice-token", ""); rec.Code != http.StatusNoContent || rec.Body.Len() != 0 || usecase.gotID != "p1" {
		t.Fatalf("delete: status = %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestPresetRoutes_InvalidBodies proves malformed JSON, unknown fields (including a
// client-supplied userId), trailing data and oversized bodies are 400 before the usecase runs,
// and `"data": null` is treated as absent.
func TestPresetRoutes_InvalidBodies(t *testing.T) {
	huge := `{"name":"x","data":{"blob":"` + strings.Repeat("a", int(presetBodyLimitBytes)) + `"}}`
	for name, body := range map[string]string{
		"malformed":       `{"name":`,
		"userId injected": `{"name":"x","data":{},"userId":"bob"}`,
		"trailing data":   `{"name":"x","data":{}}{}`,
		"oversized":       huge,
		"empty":           ``,
	} {
		usecase := &fakePresetUseCase{}
		rec := serve(usecase, "POST", "/", "alice-token", body)
		if rec.Code != http.StatusBadRequest || usecase.gotNew != nil {
			t.Fatalf("%s: status = %d, usecase reached=%v", name, rec.Code, usecase.gotNew != nil)
		}
	}

	usecase := &fakePresetUseCase{preset: &domain.Preset{ID: "p1"}}
	serve(usecase, "PATCH", "/p1", "alice-token", `{"name":"x","data":null}`)
	if usecase.gotChanges == nil || usecase.gotChanges.Data != nil {
		t.Fatalf("expected data:null to be treated as absent, got %+v", usecase.gotChanges)
	}
}

// TestPresetRoutes_ErrorMapping proves domain sentinels map to 400 / 404 / 409 with the shared
// error envelope, and an unexpected error is a 500 that leaks no internal detail.
func TestPresetRoutes_ErrorMapping(t *testing.T) {
	cases := map[string]struct {
		err  error
		code int
		want string
	}{
		"invalid":   {domain.ErrInvalidPreset, http.StatusBadRequest, "VALIDATION_ERROR"},
		"not found": {domain.ErrPresetNotFound, http.StatusNotFound, "PRESET_NOT_FOUND"},
		"limit":     {domain.ErrPresetLimitReached, http.StatusConflict, "20"},
		"internal":  {errors.New("pq: connection refused at 10.0.0.5"), http.StatusInternalServerError, "INTERNAL"},
	}
	for name, tc := range cases {
		rec := serve(&fakePresetUseCase{err: tc.err}, "GET", "/p1", "alice-token", "")
		body := rec.Body.String()
		if rec.Code != tc.code || !strings.Contains(body, tc.want) || strings.Contains(body, "10.0.0.5") {
			t.Fatalf("%s: status = %d body=%s", name, rec.Code, body)
		}
	}
}
