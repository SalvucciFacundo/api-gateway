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

// TestBucketTryAllowReportsRemaining verifies TryAllow returns the remaining
// token count after each decision, atomically with the consumption.
func TestBucketTryAllowReportsRemaining(t *testing.T) {
	// A zero refill rate removes timing-based refill noise so the remaining
	// count is exactly capacity minus consumed tokens.
	b := NewBucket(3, 0)

	for i, want := range []float64{2, 1, 0} {
		allowed, remaining := b.TryAllow()
		if !allowed {
			t.Fatalf("TryAllow() call %d = false, want true", i+1)
		}
		if remaining != want {
			t.Errorf("TryAllow() remaining call %d = %v, want %v", i+1, remaining, want)
		}
	}

	allowed, remaining := b.TryAllow()
	if allowed {
		t.Error("TryAllow() on drained bucket = true, want false")
	}
	if remaining != 0 {
		t.Errorf("TryAllow() remaining on drained bucket = %v, want 0", remaining)
	}
}

// TestBucketResetAtRefillTime verifies ResetAt returns the moment the bucket
// returns to full capacity given the current deficit and refill rate.
func TestBucketResetAtRefillTime(t *testing.T) {
	// capacity 2, refill 2/sec: a full bucket drains to zero in 1 second.
	b := NewBucket(2, 2.0)
	b.Allow()
	b.Allow()

	now := time.Now()
	b.lastRefill = now

	reset := b.ResetAt()
	want := now.Add(time.Second)
	if diff := reset.Sub(want); diff > 50*time.Millisecond || diff < -50*time.Millisecond {
		t.Errorf("ResetAt() = %v, want ~%v", reset, want)
	}
}

// TestBucketResetAtWhenFull verifies an already-full bucket resets immediately.
func TestBucketResetAtWhenFull(t *testing.T) {
	b := NewBucket(2, 2.0) // starts full

	before := time.Now()
	reset := b.ResetAt()
	if reset.Before(before) {
		t.Errorf("ResetAt() = %v, want a time at or after now", reset)
	}
}

// TestBucketLastRefill verifies LastRefill reports the last token accounting
// timestamp.
func TestBucketLastRefill(t *testing.T) {
	b := NewBucket(2, 1.0)
	marker := time.Now().Add(-30 * time.Second)
	b.lastRefill = marker

	if got := b.LastRefill(); !got.Equal(marker) {
		t.Errorf("LastRefill() = %v, want %v", got, marker)
	}
}

// TestBucketSnapshot verifies Snapshot reports capacity, current token count
// (accounting for refill), and the reset time without consuming a token.
func TestBucketSnapshot(t *testing.T) {
	b := NewBucket(5, 1.0) // capacity 5, refill 1 token/sec

	capacity, tokens, reset := b.Snapshot()
	if capacity != 5 {
		t.Errorf("capacity = %v, want 5", capacity)
	}
	if tokens != 5 {
		t.Errorf("tokens = %v, want 5 (full bucket)", tokens)
	}
	if reset.IsZero() {
		t.Error("reset is zero, want a timestamp")
	}

	if !b.Allow() {
		t.Fatal("Allow() = false, want true")
	}

	_, tokens, _ = b.Snapshot()
	// Refill accrues slightly between calls, so tokens drops from 5 to just
	// above 4 and never exceeds capacity.
	if tokens >= 5 {
		t.Errorf("tokens after Allow = %v, want < 5", tokens)
	}
	if tokens < 4 {
		t.Errorf("tokens after Allow = %v, want >= 4", tokens)
	}

	// Snapshot must be read-only: a second snapshot reports the same or more
	// tokens (refill only increases), never fewer (no consumption).
	_, tokensAgain, _ := b.Snapshot()
	if tokensAgain < tokens {
		t.Errorf("Snapshot consumed a token: %v -> %v", tokens, tokensAgain)
	}
}
