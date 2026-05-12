package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"microservie/bff/internal/handler"
	"microservie/bff/internal/middleware"
	orderv1 "microservie/proto/gen/go/order/v1"
)

type fakeOrdersClient struct {
	orders []*orderv1.Order
}

func (f *fakeOrdersClient) ListOrders(_ context.Context, _ string) ([]*orderv1.Order, error) {
	return f.orders, nil
}

func (f *fakeOrdersClient) GetOrder(_ context.Context, _, _ string) (*orderv1.Order, error) {
	if len(f.orders) == 0 {
		return nil, nil
	}
	return f.orders[0], nil
}

func TestList_returnsJSON(t *testing.T) {
	fc := &fakeOrdersClient{orders: []*orderv1.Order{
		{
			Id: "o-1", UserId: "u-1", Status: "CONFIRMED", TotalCents: 1000,
			Items: []*orderv1.OrderItem{
				{ProductId: "p-1", Quantity: 2, UnitPriceCents: 500},
			},
		},
		{
			Id: "o-2", UserId: "u-1", Status: "PENDING", TotalCents: 300,
			Items: []*orderv1.OrderItem{
				{ProductId: "p-2", Quantity: 1, UnitPriceCents: 300},
			},
		},
	}}

	h := handler.NewOrders(fc)

	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	ctx := middleware.SetUserID(req.Context(), "u-1")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Orders []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			TotalCents int32  `json:"total_cents"`
			Items      []struct {
				ProductID      string `json:"product_id"`
				Quantity       int32  `json:"quantity"`
				UnitPriceCents int32  `json:"unit_price_cents"`
			} `json:"items"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Orders) != 2 {
		t.Fatalf("want 2 orders, got %d", len(body.Orders))
	}
	if body.Orders[0].ID != "o-1" {
		t.Errorf("want first order id=o-1, got %q", body.Orders[0].ID)
	}
	if body.Orders[0].Items[0].UnitPriceCents != 500 {
		t.Errorf("want unit_price_cents=500, got %d", body.Orders[0].Items[0].UnitPriceCents)
	}
	if body.Orders[1].Status != "PENDING" {
		t.Errorf("want second order status=PENDING, got %q", body.Orders[1].Status)
	}
}
