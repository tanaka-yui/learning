package httpx

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteError_Format(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/whatever", nil)
	WriteError(w, r, 400, "INVALID_INPUT", "items required")

	if got := w.Result().StatusCode; got != 400 {
		t.Errorf("status = %d", got)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["code"] != "INVALID_INPUT" {
		t.Errorf("code = %v", body["code"])
	}
	if body["message"] != "items required" {
		t.Errorf("message = %v", body["message"])
	}
	if _, ok := body["trace_id"]; !ok {
		t.Errorf("trace_id key missing")
	}
}
