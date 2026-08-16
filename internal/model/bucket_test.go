package model

import (
	"testing"
	"time"
)

// TestBucketAllowConsumesTokens verifies RATE-007: a full bucket admits
// exactly capacity requests and then rejects the next one.
func TestBucketAllowConsumesTokens(t *testing.T) {
	b := NewBucket(3, 1.0)

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("Allow() call %d = false, want true (bucket starts full)", i+1)
		}
	}
	if b.Allow() {
		t.Error("Allow() after draining = true, want false")
	}
}

// TestBucketRefillsOverTime verifies the bucket replenishes tokens based on
// elapsed time, so a drained bucket admits requests again after a window.
func TestBucketRefillsOverTime(t *testing.T) {
	b := NewBucket(2, 100.0) // 100 tokens per second

	b.Allow()
	b.Allow()
	if b.Allow() {
		t.Error("Allow() on empty bucket = true, want false")
	}

	// Simulate one second of elapsed time without actually sleeping.
	b.lastRefill = time.Now().Add(-time.Second)

	if !b.Allow() {
		t.Error("Allow() after refill window = false, want true")
	}
}

// TestBucketRefillCappedAtCapacity verifies refill never exceeds capacity,
// so an idle bucket never accumulates more than its limit.
func TestBucketRefillCappedAtCapacity(t *testing.T) {
	b := NewBucket(1, 1000.0)

	b.Allow() // drain to zero

	// Ten seconds of idle time would add 10000 tokens if uncapped.
	b.lastRefill = time.Now().Add(-10 * time.Second)

	if !b.Allow() {
		t.Error("Allow() = false, want true")
	}
	if b.Allow() {
		t.Error("Allow() = true, want false (tokens must be capped at capacity)")
	}
}
