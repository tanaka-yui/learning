package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"microservie/bff/internal/handler"
	"microservie/bff/internal/middleware"
	catalogv1 "microservie/proto/gen/go/catalog/v1"
	orderv1 "microservie/proto/gen/go/order/v1"
)

type fakeCatalogGetter struct {
	product *catalogv1.Product
}

func (f *fakeCatalogGetter) GetProduct(_ context.Context, _ string) (*catalogv1.Product, error) {
	return f.product, nil
}

type fakeOrderPlacer struct {
	gotUserID string
	gotItems  []*orderv1.PlaceOrderItem
}

func (f *fakeOrderPlacer) PlaceOrder(_ context.Context, userID string, items []*orderv1.PlaceOrderItem) (*orderv1.PlaceOrderResponse, error) {
	f.gotUserID = userID
	f.gotItems = items
	return &orderv1.PlaceOrderResponse{OrderId: "o-1", Status: "CONFIRMED"}, nil
}

func TestCheckout_Post(t *testing.T) {
	cat := &fakeCatalogGetter{product: &catalogv1.Product{Id: "p-1", PriceCents: 500}}
	ord := &fakeOrderPlacer{}
	h := handler.NewCheckout(cat, ord)

	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]interface{}{
			{"product_id": "p-1", "quantity": 2},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := middleware.SetUserID(req.Context(), "u-1")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.Post(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		OrderID string `json:"order_id"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OrderID != "o-1" {
		t.Errorf("want order_id=o-1, got %q", resp.OrderID)
	}
	if resp.Status != "CONFIRMED" {
		t.Errorf("want status=CONFIRMED, got %q", resp.Status)
	}
	if len(ord.gotItems) != 1 {
		t.Fatalf("want 1 item sent to order service, got %d", len(ord.gotItems))
	}
	if ord.gotItems[0].UnitPriceCents != 500 {
		t.Errorf("want unit_price_cents=500, got %d", ord.gotItems[0].UnitPriceCents)
	}
}
