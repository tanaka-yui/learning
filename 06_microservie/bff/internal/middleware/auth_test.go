package middleware_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"microservie/bff/internal/middleware"
)

type fakeValidator struct {
	uid string
	err error
}

func (f *fakeValidator) ValidateToken(ctx context.Context, token string) (string, error) {
	return f.uid, f.err
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, middleware.UserID(r.Context()))
}

func TestAuth_noCookieReturns401(t *testing.T) {
	h := middleware.Auth(&fakeValidator{uid: "u-1"})(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/p", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestAuth_invalidTokenReturns401(t *testing.T) {
	h := middleware.Auth(&fakeValidator{err: fmt.Errorf("nope")})(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/p", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "bad"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestAuth_validTokenInjectsUserID(t *testing.T) {
	h := middleware.Auth(&fakeValidator{uid: "u-42"})(http.HandlerFunc(okHandler))
	req := httptest.NewRequest("GET", "/p", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "ok"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Body.String() != "u-42" {
		t.Fatalf("want 200 body=u-42, got %d body=%q", rec.Code, rec.Body.String())
	}
}
