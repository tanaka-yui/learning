package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"microservie/bff/internal/handler"
	catalogv1 "microservie/proto/gen/go/catalog/v1"
)

type fakeClient struct {
	products []*catalogv1.Product
}

func (f *fakeClient) ListProducts(ctx context.Context) ([]*catalogv1.Product, error) {
	return f.products, nil
}

func (f *fakeClient) GetProduct(_ context.Context, id string) (*catalogv1.Product, error) {
	for _, p := range f.products {
		if p.Id == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func TestListProducts_returnsJSON(t *testing.T) {
	fc := &fakeClient{products: []*catalogv1.Product{
		{Id: "a", Name: "A", PriceCents: 100},
	}}
	h := handler.NewProducts(fc)

	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var body struct {
		Products []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			PriceCents int32  `json:"price_cents"`
		} `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Products) != 1 || body.Products[0].Name != "A" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestProductsGet_OK(t *testing.T) {
	fake := &fakeClient{products: []*catalogv1.Product{
		{Id: "p-001", Name: "Tea", PriceCents: 500, Description: "Loose leaf"},
	}}
	h := handler.NewProducts(fake)

	rt := chi.NewRouter()
	rt.Get("/api/products/{id}", h.Get)

	req := httptest.NewRequest("GET", "/api/products/p-001", nil)
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		PriceCents int32  `json:"price_cents"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "p-001" || body.Name != "Tea" || body.PriceCents != 500 {
		t.Errorf("body = %+v", body)
	}
}

func TestProductsGet_NotFound(t *testing.T) {
	fake := &fakeClient{products: nil}
	h := handler.NewProducts(fake)

	rt := chi.NewRouter()
	rt.Get("/api/products/{id}", h.Get)

	req := httptest.NewRequest("GET", "/api/products/missing", nil)
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["code"] != "NOT_FOUND" {
		t.Errorf("code = %v", body["code"])
	}
}
