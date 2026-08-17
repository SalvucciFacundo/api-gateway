package middleware

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
)

// Rate limit policy constants. Capacities are per-minute totals and refill
// rates are expressed in tokens per second (capacity/60), matching the
// proposal's per-minute limits.
const (
	ipLimit         = 100.0
	ipRefillRate    = ipLimit / 60.0
	userLimit       = 60.0
	userRefillRate  = userLimit / 60.0
	loginLimit      = 10.0
	loginRefillRate = loginLimit / 60.0
)

// loginPath is the endpoint subject to the stricter anti-brute-force limit.
const loginPath = "/api/v1/auth/login"

// RateLimiter tracks one token bucket per key (RATE-007). It is safe for
// concurrent use: mu guards the map while each bucket guards its own token
// state, so bucket accounting never blocks other keys.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*model.Bucket
}

// NewRateLimiter returns an empty rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{buckets: make(map[string]*model.Bucket)}
}

// Allow reports whether the bucket for key may serve one request, creating it
// with the given capacity and refill rate on first use. It returns the
// decision and the remaining token count.
func (rl *RateLimiter) Allow(key string, capacity, refillRate float64) (bool, float64) {
	rl.mu.Lock()
	b, ok := rl.buckets[key]
	if !ok {
		b = model.NewBucket(capacity, refillRate)
		rl.buckets[key] = b
	}
	rl.mu.Unlock()

	return b.TryAllow()
}

// GetBucket returns the bucket for key, or nil if it does not exist.
func (rl *RateLimiter) GetBucket(key string) *model.Bucket {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return rl.buckets[key]
}

// Cleanup removes buckets that have not been used within maxAge, keeping the
// map bounded as addresses churn.
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, b := range rl.buckets {
		if now.Sub(b.LastRefill()) > maxAge {
			delete(rl.buckets, key)
		}
	}
}

// StartCleanup runs Cleanup on a fixed interval until ctx is cancelled. It is
// intended to be launched as a goroutine.
func (rl *RateLimiter) StartCleanup(ctx context.Context, maxAge time.Duration) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.Cleanup(maxAge)
		}
	}
}

// ipKey builds the rate limit key for an unauthenticated request from its
// client IP.
func ipKey(ip string) string { return "ip:" + ip }

// userKey builds the rate limit key for an authenticated user.
func userKey(userID string) string { return "user:" + userID }

// loginKey builds the anti-brute-force key for the login endpoint.
func loginKey(ip string) string { return "endpoint:login:" + ip }

// ClientIP extracts the remote IP from the request, stripping the port from
// RemoteAddr. It falls back to the raw address when it has no port. Exported so
// the limits status handler can report the per-IP bucket.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimit returns middleware that enforces per-IP, per-user and login
// endpoint limits, sets the X-RateLimit-* headers on every response (RATE-004)
// and returns 429 with a Retry-After header when a limit is exceeded
// (RATE-006).
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r)
			userID, hasUser := UserIDFromContext(r.Context())

			var key string
			var limit, refillRate float64

			switch {
			case r.URL.Path == loginPath:
				key = loginKey(ip)
				limit, refillRate = loginLimit, loginRefillRate
			case hasUser:
				key = userKey(userID)
				limit, refillRate = userLimit, userRefillRate
			default:
				key = ipKey(ip)
				limit, refillRate = ipLimit, ipRefillRate
			}

			allowed, remaining := limiter.Allow(key, limit, refillRate)
			// Allow always creates the bucket, so GetBucket is non-nil here.
			reset := limiter.GetBucket(key).ResetAt()

			setRateLimitHeaders(w, limit, remaining, reset)
			if !allowed {
				writeRateLimited(w, reset)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// setRateLimitHeaders writes the standard rate limit headers on the response.
func setRateLimitHeaders(w http.ResponseWriter, limit, remaining float64, reset time.Time) {
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(int64(limit), 10))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(int64(math.Floor(remaining)), 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
}

// writeRateLimited emits a 429 response with a Retry-After header indicating
// how many seconds remain until the bucket resets.
func writeRateLimited(w http.ResponseWriter, reset time.Time) {
	retryAfter := int64(math.Ceil(time.Until(reset).Seconds()))
	if retryAfter < 1 {
		retryAfter = 1
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.FormatInt(retryAfter, 10))
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
}

// BucketStatus is the public snapshot of a rate limit bucket reported by the
// limits status endpoint (RATE-005). Reset is a Unix timestamp, matching the
// X-RateLimit-Reset header convention.
type BucketStatus struct {
	Limit     float64 `json:"limit"`
	Remaining float64 `json:"remaining"`
	Reset     int64   `json:"reset"`
}

// Status reports the current state of the bucket for key. ok is false when the
// bucket has not been created yet (i.e. no request has used it).
func (rl *RateLimiter) Status(key string) (BucketStatus, bool) {
	b := rl.GetBucket(key)
	if b == nil {
		return BucketStatus{}, false
	}

	capacity, tokens, reset := b.Snapshot()
	return BucketStatus{
		Limit:     capacity,
		Remaining: math.Floor(tokens),
		Reset:     reset.Unix(),
	}, true
}

// Usage reports the per-IP and per-user rate limit status for an authenticated
// request (RATE-005). Buckets that have not been used yet are reported at full
// capacity with a reset time of now.
func (rl *RateLimiter) Usage(ip, userID string) map[string]BucketStatus {
	return map[string]BucketStatus{
		"ip":   rl.statusOrFull(ipKey(ip), ipLimit),
		"user": rl.statusOrFull(userKey(userID), userLimit),
	}
}

// statusOrFull returns the bucket's status, or a full-capacity default when the
// bucket does not exist yet.
func (rl *RateLimiter) statusOrFull(key string, limit float64) BucketStatus {
	if s, ok := rl.Status(key); ok {
		return s
	}
	return BucketStatus{Limit: limit, Remaining: limit, Reset: time.Now().Unix()}
}
