package sim

import (
	"math/rand"
	"sync"
)

type rng struct {
	mu  sync.Mutex
	src *rand.Rand
}

func newRNG(seed int64) *rng {
	return &rng{src: rand.New(rand.NewSource(seed))}
}

func (r *rng) float64() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.src.Float64()
}

func (r *rng) uniform(min, max float64) float64 {
	if max <= min {
		return min
	}
	return min + (max-min)*r.float64()
}

func (r *rng) chance(probability float64) bool {
	if probability <= 0 {
		return false
	}
	if probability >= 1 {
		return true
	}
	return r.float64() < probability
}
