package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	return "", fmt.Errorf("not found")
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
