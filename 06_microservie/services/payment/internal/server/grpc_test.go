package server_test

import (
	"context"
	"errors"
	"testing"

	"microservie/payment/internal/flake"
	"microservie/payment/internal/repo"
	"microservie/payment/internal/server"
	paymentv1 "microservie/proto/gen/go/payment/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeRepo struct {
	existing  *repo.Payment
	createErr error
	refundErr error
	created   repo.Payment
}

func (f *fakeRepo) GetByIdempotencyKey(_ context.Context, _ string) (repo.Payment, error) {
	if f.existing != nil {
		return *f.existing, nil
	}
	return repo.Payment{}, repo.ErrPaymentNotFound
}

func (f *fakeRepo) Create(_ context.Context, idemKey, orderID string, amount int32, statusStr string) (string, error) {
	if f.createErr != nil {
		return "", f.createErr
	}
	f.created = repo.Payment{
		ID:             "p-1",
		IdempotencyKey: idemKey,
		OrderID:        orderID,
		AmountCents:    amount,
		Status:         statusStr,
	}
	return "p-1", nil
}

func (f *fakeRepo) MarkRefunded(_ context.Context, _ string) error {
	return f.refundErr
}

func TestCharge_idempotentReplay(t *testing.T) {
	existing := &repo.Payment{ID: "p-old", Status: "succeeded"}
	r := &fakeRepo{existing: existing}
	// rate=0 so flake never fires; seed irrelevant
	fl := flake.New(0.0, 42)
	s := server.New(r, fl)

	req := &paymentv1.ChargeRequest{
		IdempotencyKey: "idem-1",
		OrderId:        "order-1",
		AmountCents:    1000,
	}

	res1, err := s.Charge(context.Background(), req)
	if err != nil {
		t.Fatalf("first Charge: %v", err)
	}
	if res1.PaymentId != "p-old" {
		t.Fatalf("want PaymentId=p-old, got %q", res1.PaymentId)
	}

	res2, err := s.Charge(context.Background(), req)
	if err != nil {
		t.Fatalf("second Charge: %v", err)
	}
	if res2.PaymentId != "p-old" {
		t.Fatalf("idempotent replay: want PaymentId=p-old, got %q", res2.PaymentId)
	}
}

func TestCharge_failsWhenFlaked(t *testing.T) {
	r := &fakeRepo{} // no existing — proceeds to flake check
	fl := flake.New(1.0, 42)
	s := server.New(r, fl)

	_, err := s.Charge(context.Background(), &paymentv1.ChargeRequest{
		IdempotencyKey: "idem-2",
		OrderId:        "order-2",
		AmountCents:    500,
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("want grpc status error, got %v", err)
	}
	if !errors.Is(err, err) || st.Code() != codes.Unavailable {
		t.Fatalf("want codes.Unavailable, got %v", st.Code())
	}
}
