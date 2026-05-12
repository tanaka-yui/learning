package saga

import (
	"context"
	"fmt"
)

// Item represents a line item in the order for the saga.
type Item struct {
	ProductID string
	Quantity  int32
}

// Input is the data required to run the checkout saga.
type Input struct {
	OrderID    string
	Items      []Item
	TotalCents int32
}

// Inventory is the interface the saga uses to reserve/commit/release stock.
type Inventory interface {
	Reserve(ctx context.Context, orderID string, items []Item) (reservationID string, err error)
	Commit(ctx context.Context, reservationID string) error
	Release(ctx context.Context, reservationID string) error
}

// Payment is the interface the saga uses to charge/refund.
type Payment interface {
	Charge(ctx context.Context, idempotencyKey, orderID string, amountCents int32) (paymentID string, err error)
	Refund(ctx context.Context, paymentID string) error
}

// OrderStore is the subset of repo methods required by the saga.
type OrderStore interface {
	UpdateStatus(ctx context.Context, orderID, status string) error
	LogStep(ctx context.Context, orderID, step, status, detail string) error
}

// Checkout orchestrates the checkout saga: Reserve → Charge → Commit.
// On failure at any step the completed steps are compensated in reverse.
type Checkout struct {
	inv   Inventory
	pay   Payment
	store OrderStore
}

// NewCheckout creates a new Checkout saga orchestrator.
func NewCheckout(inv Inventory, pay Payment, store OrderStore) *Checkout {
	return &Checkout{inv: inv, pay: pay, store: store}
}

// Run executes the checkout saga.
// It transitions the order to CONFIRMED on success or FAILED on any error.
func (c *Checkout) Run(ctx context.Context, in Input) error {
	// Step 1: Reserve inventory.
	resID, err := c.inv.Reserve(ctx, in.OrderID, in.Items)
	if err != nil {
		_ = c.store.LogStep(ctx, in.OrderID, "reserve", "FAILED", err.Error())
		_ = c.store.UpdateStatus(ctx, in.OrderID, "FAILED")
		return fmt.Errorf("saga reserve: %w", err)
	}
	_ = c.store.LogStep(ctx, in.OrderID, "reserve", "OK", resID)

	// Step 2: Charge payment.
	idem := "order-" + in.OrderID
	payID, err := c.pay.Charge(ctx, idem, in.OrderID, in.TotalCents)
	if err != nil {
		_ = c.store.LogStep(ctx, in.OrderID, "charge", "FAILED", err.Error())
		// Compensate: release the reservation.
		if relErr := c.inv.Release(ctx, resID); relErr != nil {
			_ = c.store.LogStep(ctx, in.OrderID, "release", "FAILED", relErr.Error())
		} else {
			_ = c.store.LogStep(ctx, in.OrderID, "release", "OK", "")
		}
		_ = c.store.UpdateStatus(ctx, in.OrderID, "FAILED")
		return fmt.Errorf("saga charge: %w", err)
	}
	_ = c.store.LogStep(ctx, in.OrderID, "charge", "OK", payID)

	// Step 3: Commit inventory reservation.
	if err := c.inv.Commit(ctx, resID); err != nil {
		_ = c.store.LogStep(ctx, in.OrderID, "commit", "FAILED", err.Error())
		// Compensate: refund the payment, then release the reservation.
		if refErr := c.pay.Refund(ctx, payID); refErr != nil {
			_ = c.store.LogStep(ctx, in.OrderID, "refund", "FAILED", refErr.Error())
		} else {
			_ = c.store.LogStep(ctx, in.OrderID, "refund", "OK", "")
		}
		if relErr := c.inv.Release(ctx, resID); relErr != nil {
			_ = c.store.LogStep(ctx, in.OrderID, "release", "FAILED", relErr.Error())
		} else {
			_ = c.store.LogStep(ctx, in.OrderID, "release", "OK", "")
		}
		_ = c.store.UpdateStatus(ctx, in.OrderID, "FAILED")
		return fmt.Errorf("saga commit: %w", err)
	}
	_ = c.store.LogStep(ctx, in.OrderID, "commit", "OK", "")

	// All steps succeeded.
	_ = c.store.UpdateStatus(ctx, in.OrderID, "CONFIRMED")
	return nil
}
