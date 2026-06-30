package main

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"
)

func TestEchoVersion(t *testing.T) {
	os.Setenv("APP_VERSION", "v1")
	w := httptest.NewRecorder()
	echo(w, httptest.NewRequest("GET", "/api/v1/echo", nil))
	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got map[string]string
	_ = json.NewDecoder(w.Result().Body).Decode(&got)
	if got["version"] != "v1" {
		t.Fatalf("want v1, got %s", got["version"])
	}
}
