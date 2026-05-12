package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"microservie/bff/internal/handler"
	catalogv1 "microservie/proto/gen/go/catalog/v1"
)

type fakeClient struct {
	products []*catalogv1.Product
}

func (f *fakeClient) ListProducts(ctx context.Context) ([]*catalogv1.Product, error) {
	return f.products, nil
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
