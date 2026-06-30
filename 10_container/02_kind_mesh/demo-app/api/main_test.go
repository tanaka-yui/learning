package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestEchoVersionFromEnv(t *testing.T) {
	t.Setenv("APP_VERSION", "v9")
	w := httptest.NewRecorder()
	echo(w, httptest.NewRequest("GET", "/api/v1/echo", nil))
	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got map[string]string
	_ = json.NewDecoder(w.Result().Body).Decode(&got)
	if got["version"] != "v9" {
		t.Fatalf("want v9, got %s", got["version"])
	}
}
