package checkout

import (
	"context"
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	if err := Validate(Request{Item: "book", Qty: 1}); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if err := Validate(Request{Item: "", Qty: 1}); err == nil {
		t.Fatal("empty item should be rejected")
	}
	if err := Validate(Request{Item: "book", Qty: 0}); err == nil {
		t.Fatal("zero qty should be rejected")
	}
}

func TestService_Checkout_Success(t *testing.T) {
	svc := NewService(0.0)
	res, err := svc.Checkout(context.Background(), Request{Item: "book", Qty: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OrderID == "" {
		t.Fatal("expected an order id")
	}
}

func TestService_Checkout_FlakeAlwaysFails(t *testing.T) {
	svc := NewService(1.0)
	_, err := svc.Checkout(context.Background(), Request{Item: "book", Qty: 2})
	if !errors.Is(err, ErrChargeFailed) {
		t.Fatalf("expected ErrChargeFailed, got %v", err)
	}
}
