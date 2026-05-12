package saga_test

import (
	"context"
	"errors"
	"testing"

	"microservie/order/internal/saga"
)

// fakeInventory implements saga.Inventory.
type fakeInventory struct {
	reserveID  string
	reserveErr error
	commitErr  error
	releaseErr error
}

func (f *fakeInventory) Reserve(_ context.Context, _ string, _ []saga.Item) (string, error) {
	return f.reserveID, f.reserveErr
}
func (f *fakeInventory) Commit(_ context.Context, _ string) error { return f.commitErr }
func (f *fakeInventory) Release(_ context.Context, _ string) error { return f.releaseErr }

// fakePayment implements saga.Payment.
type fakePayment struct {
	paymentID string
	chargeErr error
	refundErr error
}

func (f *fakePayment) Charge(_ context.Context, _, _ string, _ int32) (string, error) {
	return f.paymentID, f.chargeErr
}
func (f *fakePayment) Refund(_ context.Context, _ string) error { return f.refundErr }

// fakeOrderStore implements saga.OrderStore.
type fakeOrderStore struct {
	statusHistory []string
	logHistory    []string
	updateErr     error
	logErr        error
}

func (f *fakeOrderStore) UpdateStatus(_ context.Context, _ string, s string) error {
	f.statusHistory = append(f.statusHistory, s)
	return f.updateErr
}
func (f *fakeOrderStore) LogStep(_ context.Context, _, step, status, _ string) error {
	f.logHistory = append(f.logHistory, step+":"+status)
	return f.logErr
}

func makeItems() []saga.Item {
	return []saga.Item{{ProductID: "p-1", Quantity: 2}}
}

func makeInput(orderID string) saga.Input {
	return saga.Input{OrderID: orderID, Items: makeItems(), TotalCents: 1000}
}

// Test 1: happy path — Reserve → Charge → Commit → status=CONFIRMED
func TestCheckout_happyPath(t *testing.T) {
	inv := &fakeInventory{reserveID: "res-1"}
	pay := &fakePayment{paymentID: "pay-1"}
	store := &fakeOrderStore{}

	c := saga.NewCheckout(inv, pay, store)
	if err := c.Run(context.Background(), makeInput("order-1")); err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}

	if len(store.statusHistory) == 0 || store.statusHistory[len(store.statusHistory)-1] != "CONFIRMED" {
		t.Fatalf("want final status CONFIRMED, got %v", store.statusHistory)
	}
}

// Test 2: Reserve fails — saga returns error, status=FAILED, no compensation needed
func TestCheckout_reserveFail(t *testing.T) {
	reserveErr := errors.New("out of stock")
	inv := &fakeInventory{reserveErr: reserveErr}
	pay := &fakePayment{}
	store := &fakeOrderStore{}

	c := saga.NewCheckout(inv, pay, store)
	if err := c.Run(context.Background(), makeInput("order-2")); err == nil {
		t.Fatal("Run: expected error, got nil")
	}

	if len(store.statusHistory) == 0 || store.statusHistory[len(store.statusHistory)-1] != "FAILED" {
		t.Fatalf("want final status FAILED, got %v", store.statusHistory)
	}
	// Payment Charge should never have been called (paymentID never set)
	if pay.chargeErr != nil {
		t.Fatal("unexpected charge call")
	}
}

// Test 3: Charge fails — saga compensates by releasing reservation, status=FAILED
func TestCheckout_chargeFailWithRelease(t *testing.T) {
	chargeErr := errors.New("payment declined")
	inv := &fakeInventory{reserveID: "res-2"}
	pay := &fakePayment{chargeErr: chargeErr}
	store := &fakeOrderStore{}

	c := saga.NewCheckout(inv, pay, store)
	if err := c.Run(context.Background(), makeInput("order-3")); err == nil {
		t.Fatal("Run: expected error, got nil")
	}

	if len(store.statusHistory) == 0 || store.statusHistory[len(store.statusHistory)-1] != "FAILED" {
		t.Fatalf("want final status FAILED, got %v", store.statusHistory)
	}

	// Verify Release was logged
	found := false
	for _, l := range store.logHistory {
		if l == "release:OK" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected release:OK in log, got %v", store.logHistory)
	}
}

// Test 4: Commit fails — saga compensates by refunding payment and releasing reservation, status=FAILED
func TestCheckout_commitFailWithRefund(t *testing.T) {
	commitErr := errors.New("commit failed")
	inv := &fakeInventory{reserveID: "res-3", commitErr: commitErr}
	pay := &fakePayment{paymentID: "pay-2"}
	store := &fakeOrderStore{}

	c := saga.NewCheckout(inv, pay, store)
	if err := c.Run(context.Background(), makeInput("order-4")); err == nil {
		t.Fatal("Run: expected error, got nil")
	}

	if len(store.statusHistory) == 0 || store.statusHistory[len(store.statusHistory)-1] != "FAILED" {
		t.Fatalf("want final status FAILED, got %v", store.statusHistory)
	}

	// Verify Refund was logged
	foundRefund := false
	for _, l := range store.logHistory {
		if l == "refund:OK" {
			foundRefund = true
		}
	}
	if !foundRefund {
		t.Fatalf("expected refund:OK in log, got %v", store.logHistory)
	}
}
