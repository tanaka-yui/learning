package flake

import (
	"math/rand"
	"sync"
)

type Flake struct {
	rate float64
	rng  *rand.Rand
	mu   sync.Mutex
}

func New(rate float64, seed int64) *Flake {
	return &Flake{rate: rate, rng: rand.New(rand.NewSource(seed))}
}

func (f *Flake) ShouldFail() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rng.Float64() < f.rate
}
