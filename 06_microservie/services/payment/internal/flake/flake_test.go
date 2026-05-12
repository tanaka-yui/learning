package flake_test

import (
	"testing"

	"microservie/payment/internal/flake"
)

func TestShouldFail_zeroRateNeverFails(t *testing.T) {
	f := flake.New(0.0, 42)
	for i := 0; i < 100; i++ {
		if f.ShouldFail() {
			t.Fatalf("rate=0 returned true at iter %d", i)
		}
	}
}

func TestShouldFail_fullRateAlwaysFails(t *testing.T) {
	f := flake.New(1.0, 42)
	for i := 0; i < 100; i++ {
		if !f.ShouldFail() {
			t.Fatalf("rate=1 returned false at iter %d", i)
		}
	}
}

func TestShouldFail_approximateRate(t *testing.T) {
	f := flake.New(0.3, 42) // 同じ seed で再現性
	fails := 0
	const n = 10000
	for i := 0; i < n; i++ {
		if f.ShouldFail() {
			fails++
		}
	}
	rate := float64(fails) / float64(n)
	if rate < 0.27 || rate > 0.33 {
		t.Fatalf("want ~0.30, got %.3f", rate)
	}
}
