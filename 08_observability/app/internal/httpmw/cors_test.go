package httpmw

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_PreflightShortCircuits(t *testing.T) {
	h := CORS("http://localhost:5174")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("preflight OPTIONS must not reach the next handler")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/checkout", nil))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("want 204 for preflight, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5174" {
		t.Fatalf("missing/incorrect Allow-Origin: %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("preflight must advertise allowed headers (Content-Type/traceparent)")
	}
}

func TestCORS_ActualRequestPassesThroughWithHeader(t *testing.T) {
	called := false
	h := CORS("http://localhost:5174")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/checkout", nil))

	if !called {
		t.Fatal("POST must reach the next handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5174" {
		t.Fatalf("actual response missing Allow-Origin: %q", got)
	}
}
