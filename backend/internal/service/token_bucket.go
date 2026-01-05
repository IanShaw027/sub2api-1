package service

import (
	"sync"
	"time"
)

// tokenBucket is a simple in-memory token bucket rate limiter.
//
// Notes:
// - It is intentionally process-local. For Ops alerting we already use a leader lock,
//   so a local limiter protects the SMTP provider without requiring distributed state.
// - The bucket refills continuously at refillPerSecond.
type tokenBucket struct {
	mu              sync.Mutex
	capacity        float64
	tokens          float64
	refillPerSecond float64
	lastRefill      time.Time
}

func newTokenBucket(refillPerSecond float64, capacity float64) *tokenBucket {
	if refillPerSecond < 0 {
		refillPerSecond = 0
	}
	if capacity <= 0 {
		capacity = 1
	}
	now := time.Now()
	return &tokenBucket{
		capacity:        capacity,
		tokens:          capacity,
		refillPerSecond: refillPerSecond,
		lastRefill:      now,
	}
}

func (b *tokenBucket) allow(cost float64) bool {
	if b == nil {
		return true
	}
	if cost <= 0 {
		return true
	}

	now := time.Now()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill.
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 && b.refillPerSecond > 0 {
		b.tokens += elapsed * b.refillPerSecond
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.lastRefill = now
	} else if elapsed > 0 {
		b.lastRefill = now
	}

	if b.tokens < cost {
		return false
	}
	b.tokens -= cost
	return true
}

