package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	healthz(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	body, _ := io.ReadAll(w.Result().Body)
	if string(body) != "ok" {
		t.Fatalf("want body ok, got %q", string(body))
	}
}
