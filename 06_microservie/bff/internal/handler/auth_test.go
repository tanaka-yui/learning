package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"microservie/bff/internal/handler"
	"microservie/bff/internal/middleware"
)

type fakeProfile struct {
	profiles map[string]string // user_id -> email
}

func (f *fakeProfile) SignUp(_ context.Context, _, _ string) (string, error) { return "", nil }
func (f *fakeProfile) SignIn(_ context.Context, _, _ string) (string, error) { return "", nil }
func (f *fakeProfile) GetUser(_ context.Context, id string) (string, error) {
	if email, ok := f.profiles[id]; ok {
		return email, nil
	}
	return "", status.Error(codes.NotFound, "not found")
}

func TestAuthMe_OK(t *testing.T) {
	fake := &fakeProfile{profiles: map[string]string{"u-1": "alice@example.com"}}
	h := handler.NewAuth(fake)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	ctx := middleware.SetUserID(req.Context(), "u-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Me(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.UserID != "u-1" || body.Email != "alice@example.com" {
		t.Errorf("body = %+v", body)
	}
}

func TestAuthSignOut_ClearsCookie(t *testing.T) {
	h := handler.NewAuth(&fakeProfile{})

	req := httptest.NewRequest("POST", "/api/auth/signout", nil)
	w := httptest.NewRecorder()
	h.SignOut(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d", w.Code)
	}
	sc := w.Header().Get("Set-Cookie")
	if !strings.Contains(sc, "session=") || !strings.Contains(sc, "Max-Age=0") {
		t.Errorf("Set-Cookie = %q", sc)
	}
}

func TestAuthMe_NotFoundReturns401(t *testing.T) {
	fake := &fakeProfile{profiles: map[string]string{}} // empty
	h := handler.NewAuth(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	ctx := middleware.SetUserID(req.Context(), "u-unknown")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Me(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestAuthMe_UpstreamFailureReturns502(t *testing.T) {
	fake := &fakeProfileErr{err: errors.New("dial tcp: connection refused")}
	h := handler.NewAuth(fake)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	ctx := middleware.SetUserID(req.Context(), "u-1")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	h.Me(w, req)
	if w.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "UPSTREAM_FAILED" {
		t.Errorf("code = %v, want UPSTREAM_FAILED", body["code"])
	}
}

type fakeProfileErr struct{ err error }

func (f *fakeProfileErr) SignUp(_ context.Context, _, _ string) (string, error) { return "", nil }
func (f *fakeProfileErr) SignIn(_ context.Context, _, _ string) (string, error) { return "", nil }
func (f *fakeProfileErr) GetUser(_ context.Context, _ string) (string, error)   { return "", f.err }

type fakeProfileSignIn struct {
	fakeProfile
	signInToken string
	signInErr   error
}

func (f *fakeProfileSignIn) SignIn(_ context.Context, _, _ string) (string, error) {
	return f.signInToken, f.signInErr
}

func TestAuthSignIn_SetsCookieOnSuccess(t *testing.T) {
	fake := &fakeProfileSignIn{signInToken: "jwt-xyz"}
	h := handler.NewAuth(fake)

	body := strings.NewReader(`{"Email":"alice@example.com","Password":"password"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signin", body)
	w := httptest.NewRecorder()
	h.SignIn(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	sc := w.Header().Get("Set-Cookie")
	if !strings.Contains(sc, "session=jwt-xyz") {
		t.Errorf("Set-Cookie = %q", sc)
	}
}

func TestAuthSignIn_InvalidCredentialsReturns401(t *testing.T) {
	fake := &fakeProfileSignIn{signInErr: errors.New("invalid")}
	h := handler.NewAuth(fake)

	body := strings.NewReader(`{"Email":"x","Password":"y"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signin", body)
	w := httptest.NewRecorder()
	h.SignIn(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

type fakeProfileSignUp struct {
	fakeProfile
	signUpID  string
	signUpErr error
}

func (f *fakeProfileSignUp) SignUp(_ context.Context, _, _ string) (string, error) {
	return f.signUpID, f.signUpErr
}

func TestAuthSignUp_ReturnsUserID(t *testing.T) {
	fake := &fakeProfileSignUp{signUpID: "u-42"}
	h := handler.NewAuth(fake)

	body := strings.NewReader(`{"Email":"new@example.com","Password":"pw"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/signup", body)
	w := httptest.NewRecorder()
	h.SignUp(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["user_id"] != "u-42" {
		t.Errorf("user_id = %q, want u-42", resp["user_id"])
	}
}
