package server_test

import (
	"context"
	"errors"
	"testing"

	"microservie/order/internal/repo"
	"microservie/order/internal/saga"
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

type fakeSaga struct{ err error }

func (f *fakeSaga) Run(_ context.Context, _ saga.Input) error { return f.err }

func TestPlaceOrder_happyReturnsConfirmed(t *testing.T) {
	r := &fakeRepo{orderID: "o-1", totalCents: 1000}
	sg := &fakeSaga{err: nil}
	s := server.New(r, sg)

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
	if res.Status != "CONFIRMED" {
		t.Fatalf("want Status=CONFIRMED, got %q", res.Status)
	}
}

func TestPlaceOrder_sagaFailureReturnsFailed(t *testing.T) {
	r := &fakeRepo{orderID: "o-2", totalCents: 2000}
	sg := &fakeSaga{err: errors.New("saga failed")}
	s := server.New(r, sg)

	req := &orderv1.PlaceOrderRequest{
		UserId: "user-2",
		Items: []*orderv1.PlaceOrderItem{
			{ProductId: "prod-2", Quantity: 1, UnitPriceCents: 2000},
		},
	}

	res, err := s.PlaceOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("PlaceOrder: unexpected gRPC error: %v", err)
	}
	if res.OrderId != "o-2" {
		t.Fatalf("want OrderId=o-2, got %q", res.OrderId)
	}
	if res.Status != "FAILED" {
		t.Fatalf("want Status=FAILED, got %q", res.Status)
	}
}
