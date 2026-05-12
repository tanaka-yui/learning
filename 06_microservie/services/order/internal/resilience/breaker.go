package resilience

import (
	"time"

	"github.com/sony/gobreaker"
)

// NewBreaker creates a new circuit breaker with the given name.
// Settings:
//   - MaxRequests: 1 trial request allowed in half-open state
//   - Interval: 10s counter reset interval (closed state)
//   - Timeout: 30s open state duration before transitioning to half-open
//   - ReadyToTrip: opens after >= 5 requests with >= 50% failure rate
func NewBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1,                // half-open trial requests
		Interval:    10 * time.Second, // counter reset interval
		Timeout:     30 * time.Second, // open state duration
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.Requests >= 5 && float64(c.TotalFailures)/float64(c.Requests) >= 0.5
		},
	})
}
