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

// Allow reports whether one request may proceed, consuming a token if so.
//
// Before deciding, it credits the bucket with the tokens that accumulated
// since lastRefill, capped at capacity. A bucket with fewer than one token
// rejects the request.
func (b *Bucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens = min(b.capacity, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
