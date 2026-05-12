package resilience_test

import (
	"errors"
	"testing"

	"microservie/order/internal/resilience"

	"github.com/sony/gobreaker"
)

// openBreaker is a helper that drives the breaker into StateOpen by
// calling Execute with an always-failing function until the breaker trips.
func openBreaker(b *gobreaker.CircuitBreaker) {
	failFn := func() (interface{}, error) {
		return nil, errors.New("simulated failure")
	}
	// Call at least 6 times — ReadyToTrip requires >= 5 requests AND >= 50% failures.
	for range 6 {
		_, _ = b.Execute(failFn)
	}
}

// TestBreaker_opensAfterRepeatedFailures verifies that after enough failures
// the circuit breaker transitions to StateOpen.
func TestBreaker_opensAfterRepeatedFailures(t *testing.T) {
	b := resilience.NewBreaker("test-open")
	openBreaker(b)

	if got := b.State(); got != gobreaker.StateOpen {
		t.Fatalf("expected StateOpen after repeated failures, got %v", got)
	}
}

// TestBreaker_openStateShortCircuits verifies that when the breaker is open,
// Execute immediately returns ErrOpenState without calling the supplied function.
func TestBreaker_openStateShortCircuits(t *testing.T) {
	b := resilience.NewBreaker("test-short-circuit")
	openBreaker(b)

	counter := 0
	_, err := b.Execute(func() (interface{}, error) {
		counter++
		return nil, nil
	})

	if counter != 0 {
		t.Fatalf("expected counter=0 (short-circuit), got %d", counter)
	}
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("expected ErrOpenState, got %v", err)
	}
}
