package checkout

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_Success(t *testing.T) {
	h := NewHandler(NewService(0.0))
	req := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(`{"item":"book","qty":1}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "order_id") {
		t.Fatalf("missing order_id: %s", rec.Body.String())
	}
}

func TestHandler_InvalidRequest(t *testing.T) {
	h := NewHandler(NewService(0.0))
	req := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(`{"item":"","qty":0}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestHandler_ChargeFailure(t *testing.T) {
	h := NewHandler(NewService(1.0))
	req := httptest.NewRequest(http.MethodPost, "/api/checkout", strings.NewReader(`{"item":"book","qty":1}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d", rec.Code)
	}
}
