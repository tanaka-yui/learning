package server_test

import (
	"context"
	"testing"

	"microservie/order/internal/repo"
	"microservie/order/internal/server"
	orderv1 "microservie/proto/gen/go/order/v1"
)

type fakeRepo struct {
	orderID    string
	totalCents int32
	createErr  error
}

func (f *fakeRepo) Create(_ context.Context, _ string, _ []repo.OrderItem) (string, int32, error) {
	return f.orderID, f.totalCents, f.createErr
}

func (f *fakeRepo) UpdateStatus(_ context.Context, _, _ string) error { return nil }

func (f *fakeRepo) Get(_ context.Context, _, _ string) (repo.Order, error) {
	return repo.Order{}, repo.ErrOrderNotFound
}

func (f *fakeRepo) List(_ context.Context, _ string) ([]repo.Order, error) {
	return nil, nil
}

func (f *fakeRepo) LogStep(_ context.Context, _, _, _, _ string) error { return nil }

func TestPlaceOrder_returnsPending(t *testing.T) {
	r := &fakeRepo{orderID: "o-1", totalCents: 1000}
	s := server.New(r)

	req := &orderv1.PlaceOrderRequest{
		UserId: "user-1",
		Items: []*orderv1.PlaceOrderItem{
			{ProductId: "prod-1", Quantity: 2, UnitPriceCents: 500},
		},
	}

	res, err := s.PlaceOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if res.OrderId != "o-1" {
		t.Fatalf("want OrderId=o-1, got %q", res.OrderId)
	}
	if res.Status != "PENDING" {
		t.Fatalf("want Status=PENDING, got %q", res.Status)
	}
}
