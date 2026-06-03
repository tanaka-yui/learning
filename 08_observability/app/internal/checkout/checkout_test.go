package checkout

import (
	"context"
	"errors"
	"testing"
	"time"
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

func TestService_Checkout_InjectsLatency(t *testing.T) {
	svc := NewService(0.0).WithLatency(200 * time.Millisecond)
	var slept time.Duration
	svc.sleep = func(d time.Duration) { slept = d }
	if _, err := svc.Checkout(context.Background(), Request{Item: "book", Qty: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slept != 200*time.Millisecond {
		t.Fatalf("want 200ms injected, got %v", slept)
	}
}

func TestService_Checkout_NoLatencyByDefault(t *testing.T) {
	svc := NewService(0.0)
	called := false
	svc.sleep = func(time.Duration) { called = true }
	if _, err := svc.Checkout(context.Background(), Request{Item: "book", Qty: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("sleep must not be called when latency is 0")
	}
}
