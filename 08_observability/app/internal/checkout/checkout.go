// Package checkout is the demo domain: a 3-step checkout whose final
// charge step fails with a configurable probability so the observability
// stack has interesting traces, error metrics, and error logs to show.
package checkout

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

type Request struct {
	Item string `json:"item"`
	Qty  int    `json:"qty"`
}

type Result struct {
	OrderID string `json:"order_id"`
}

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrChargeFailed   = errors.New("charge failed")
)

func Validate(r Request) error {
	if r.Item == "" {
		return fmt.Errorf("%w: item is required", ErrInvalidRequest)
	}
	if r.Qty <= 0 {
		return fmt.Errorf("%w: qty must be > 0", ErrInvalidRequest)
	}
	return nil
}

type Service struct {
	flakeRate float64
	latency   time.Duration
	rng       func() float64
	sleep     func(time.Duration)
}

func NewService(flakeRate float64) *Service {
	return &Service{flakeRate: flakeRate, rng: rand.Float64, sleep: time.Sleep}
}

// WithLatency makes every Checkout sleep d, so the latency alert can be demoed.
func (s *Service) WithLatency(d time.Duration) *Service {
	s.latency = d
	return s
}

func (s *Service) Checkout(ctx context.Context, r Request) (Result, error) {
	if s.latency > 0 {
		s.sleep(s.latency)
	}
	if err := Validate(r); err != nil {
		return Result{}, err
	}
	if err := s.reserveStock(ctx, r); err != nil {
		return Result{}, err
	}
	if err := s.charge(ctx, r); err != nil {
		return Result{}, err
	}
	return Result{OrderID: fmt.Sprintf("ord-%s-%d", r.Item, r.Qty)}, nil
}

func (s *Service) reserveStock(_ context.Context, _ Request) error {
	return nil
}

func (s *Service) charge(_ context.Context, _ Request) error {
	if s.rng() < s.flakeRate {
		return ErrChargeFailed
	}
	return nil
}
