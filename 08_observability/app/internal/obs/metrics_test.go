package obs

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestREDMiddleware_PassesThrough(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	mw, err := NewREDMiddleware("test")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status not preserved: %d", rec.Code)
	}
}
