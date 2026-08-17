package model

import (
	"sync"
	"time"
)

// Bucket is a token bucket rate limiter.
//
// tokens holds the currently available tokens, capacity the maximum, and
// refillRate the replenishment speed in tokens per second. lastRefill is the
// timestamp of the most recent token accounting. All fields are guarded by mu
// so a single bucket is safe for concurrent Allow calls.
type Bucket struct {
	mu         sync.Mutex
	tokens     float64
	capacity   float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewBucket returns a full bucket with the given capacity and refill rate.
func NewBucket(capacity, refillRate float64) *Bucket {
	return &Bucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// TryAllow reports whether one request may proceed, consuming a token if so,
// and returns the remaining token count after the decision. It is the atomic
// primitive behind Allow.
func (b *Bucket) TryAllow() (bool, float64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.capacity, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens < 1 {
		return false, b.tokens
	}
	b.tokens--
	return true, b.tokens
}

// Allow reports whether one request may proceed, consuming a token if so.
//
// Before deciding, it credits the bucket with the tokens that accumulated
// since lastRefill, capped at capacity. A bucket with fewer than one token
// rejects the request.
func (b *Bucket) Allow() bool {
	allowed, _ := b.TryAllow()
	return allowed
}

// LastRefill returns the timestamp of the most recent token accounting, which
// is also the last time the bucket was used. Cleanup uses it to age out idle
// buckets.
func (b *Bucket) LastRefill() time.Time {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastRefill
}

// ResetAt returns the time at which the bucket will be full again, assuming no
// further requests. A bucket already at capacity resets immediately.
func (b *Bucket) ResetAt() time.Time {
	_, _, reset := b.Snapshot()
	return reset
}

// Snapshot reports the bucket's capacity, current token count (after crediting
// the refill accumulated since last use), and the time it returns to full. It
// is a read-only view: no token is consumed. All three values are read under a
// single lock so the snapshot is internally consistent. The middleware exposes
// this through its status endpoint (RATE-005).
func (b *Bucket) Snapshot() (capacity, tokens float64, reset time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	tokens = min(b.capacity, b.tokens+elapsed*b.refillRate)
	if tokens >= b.capacity {
		return b.capacity, tokens, now
	}
	deficit := b.capacity - tokens
	secondsToFull := deficit / b.refillRate
	return b.capacity, tokens, now.Add(time.Duration(secondsToFull * float64(time.Second)))
}
