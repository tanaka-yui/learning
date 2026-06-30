package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestEcho(t *testing.T) {
	w := httptest.NewRecorder()
	echo(w, httptest.NewRequest("GET", "/api/v1/echo", nil))
	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var got map[string]string
	if err := json.NewDecoder(w.Result().Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["lang"] != "go" {
		t.Fatalf("want go, got %s", got["lang"])
	}
}
