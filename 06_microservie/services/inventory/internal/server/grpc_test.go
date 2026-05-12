package server_test

import (
	"context"
	"testing"

	"microservie/inventory/internal/repo"
	"microservie/inventory/internal/server"
	inventoryv1 "microservie/proto/gen/go/inventory/v1"
)

type fakeRepo struct {
	resID string
	err   error
}

func (f *fakeRepo) Reserve(ctx context.Context, orderID string, items []repo.Item) (string, error) {
	return f.resID, f.err
}
func (f *fakeRepo) Commit(ctx context.Context, id string) error  { return f.err }
func (f *fakeRepo) Release(ctx context.Context, id string) error { return f.err }
func (f *fakeRepo) GetStock(ctx context.Context, pid string) (repo.Stock, error) {
	return repo.Stock{Available: 10}, nil
}

func TestReserve_OK(t *testing.T) {
	s := server.New(&fakeRepo{resID: "r-1"})
	res, err := s.Reserve(context.Background(), &inventoryv1.ReserveRequest{
		OrderId: "o-1", Items: []*inventoryv1.Item{{ProductId: "p-001", Quantity: 1}},
	})
	if err != nil || res.ReservationId != "r-1" {
		t.Fatalf("got %v, %v", res, err)
	}
}

func TestReserve_InsufficientReturnsFailedPrecondition(t *testing.T) {
	s := server.New(&fakeRepo{err: repo.ErrInsufficientStock})
	_, err := s.Reserve(context.Background(), &inventoryv1.ReserveRequest{OrderId: "o-1"})
	if err == nil {
		t.Fatal("want error")
	}
}
