package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// TestRateLimitKeys verifies the three key shapes isolate IP, user and login
// endpoint buckets from each other (RATE-008).
func TestRateLimitKeys(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) string
		in   string
		want string
	}{
		{"ip key", ipKey, "1.2.3.4", "ip:1.2.3.4"},
		{"user key", userKey, "user-9", "user:user-9"},
		{"login key", loginKey, "1.2.3.4", "endpoint:login:1.2.3.4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.in); got != tt.want {
				t.Errorf("key(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestClientIP verifies the remote address is reduced to its host component.
func TestClientIP(t *testing.T) {
	tests := []struct {
		name   string
		remote string
		want   string
	}{
		{"host and port", "203.0.113.7:4567", "203.0.113.7"},
		{"ipv6", "[2001:db8::1]:8080", "2001:db8::1"},
		{"no port", "203.0.113.7", "203.0.113.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			if got := ClientIP(req); got != tt.want {
				t.Errorf("ClientIP(%q) = %q, want %q", tt.remote, got, tt.want)
			}
		})
	}
}

// TestRateLimiterAllowCreatesAndDrains verifies Allow creates a bucket on
// first use and reports the decision plus remaining tokens.
func TestRateLimiterAllowCreatesAndDrains(t *testing.T) {
	rl := NewRateLimiter()
	for i := 0; i < 3; i++ {
		allowed, _ := rl.Allow("ip:1.2.3.4", 3, 0)
		if !allowed {
			t.Fatalf("Allow() call %d = false, want true", i+1)
		}
	}
	allowed, remaining := rl.Allow("ip:1.2.3.4", 3, 0)
	if allowed {
		t.Error("Allow() after drain = true, want false")
	}
	if remaining != 0 {
		t.Errorf("remaining = %v, want 0", remaining)
	}
}

// TestRateLimiterGetBucket verifies GetBucket returns nil before first use and
// the bucket afterwards.
func TestRateLimiterGetBucket(t *testing.T) {
	rl := NewRateLimiter()
	if got := rl.GetBucket("ip:1.2.3.4"); got != nil {
		t.Error("GetBucket() before Allow = non-nil, want nil")
	}
	rl.Allow("ip:1.2.3.4", 5, 1.0)
	if got := rl.GetBucket("ip:1.2.3.4"); got == nil {
		t.Error("GetBucket() after Allow = nil, want bucket")
	}
}

// TestRateLimiterCleanupRemovesIdleBuckets verifies Cleanup drops buckets that
// are older than maxAge.
func TestRateLimiterCleanupRemovesIdleBuckets(t *testing.T) {
	rl := NewRateLimiter()
	rl.Allow("ip:1.2.3.4", 5, 1.0)
	rl.Allow("ip:5.6.7.8", 5, 1.0)

	rl.Cleanup(0) // any elapsed time exceeds maxAge 0

	if got := rl.GetBucket("ip:1.2.3.4"); got != nil {
		t.Error("GetBucket() after Cleanup(0) = non-nil, want nil")
	}
	if got := rl.GetBucket("ip:5.6.7.8"); got != nil {
		t.Error("GetBucket() after Cleanup(0) = non-nil, want nil")
	}
}

// TestRateLimiterCleanupKeepsActiveBuckets verifies Cleanup with a large maxAge
// leaves recently-used buckets in place.
func TestRateLimiterCleanupKeepsActiveBuckets(t *testing.T) {
	rl := NewRateLimiter()
	rl.Allow("ip:1.2.3.4", 5, 1.0)

	rl.Cleanup(time.Hour)

	if got := rl.GetBucket("ip:1.2.3.4"); got == nil {
		t.Error("GetBucket() after Cleanup(1h) = nil, want bucket")
	}
}

// TestRateLimiterStartCleanupStopsOnContextCancel verifies StartCleanup returns
// promptly when its context is cancelled.
func TestRateLimiterStartCleanupStopsOnContextCancel(t *testing.T) {
	rl := NewRateLimiter()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		rl.StartCleanup(ctx, time.Minute)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartCleanup did not return after context cancellation")
	}
}

// TestRateLimitSetsHeadersAndPasses verifies RATE-004: a successful request
// carries X-RateLimit-Limit/Remaining/Reset headers, and the IP bucket allows
// requests within its bound.
func TestRateLimitSetsHeadersAndPasses(t *testing.T) {
	rl := NewRateLimiter()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits/status", nil)
	req.RemoteAddr = "203.0.113.7:4567"
	rec := httptest.NewRecorder()

	RateLimit(rl)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "100" {
		t.Errorf("X-RateLimit-Limit = %q, want 100", got)
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "99" {
		t.Errorf("X-RateLimit-Remaining = %q, want 99", got)
	}
	if got := rec.Header().Get("X-RateLimit-Reset"); got == "" {
		t.Error("X-RateLimit-Reset is empty, want a Unix timestamp")
	}
}

// TestRateLimitReturns429WhenExceeded verifies RATE-006: exceeding the IP
// limit yields 429 with a Retry-After header.
func TestRateLimitReturns429WhenExceeded(t *testing.T) {
	rl := NewRateLimiter()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Drain the IP bucket for 203.0.113.7.
	for i := 0; i < int(ipLimit); i++ {
		rl.Allow(ipKey("203.0.113.7"), ipLimit, ipRefillRate)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.RemoteAddr = "203.0.113.7:4567"
	rec := httptest.NewRecorder()

	RateLimit(rl)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	retryAfter := rec.Header().Get("Retry-After")
	n, err := strconv.Atoi(retryAfter)
	if err != nil || n < 1 {
		t.Errorf("Retry-After = %q, want a positive integer", retryAfter)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

// TestRateLimitLoginEndpointUsesLoginBucket verifies RATE-003: the login
// endpoint is limited by its own 10/min bucket.
func TestRateLimitLoginEndpointUsesLoginBucket(t *testing.T) {
	rl := NewRateLimiter()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < int(loginLimit); i++ {
		rl.Allow(loginKey("203.0.113.7"), loginLimit, loginRefillRate)
	}

	req := httptest.NewRequest(http.MethodPost, loginPath, nil)
	req.RemoteAddr = "203.0.113.7:4567"
	rec := httptest.NewRecorder()
	RateLimit(rl)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "10" {
		t.Errorf("X-RateLimit-Limit = %q, want 10", got)
	}
}

// TestRateLimitUserBucket verifies RATE-002: an authenticated request is
// limited by the user bucket (60/min) rather than the IP bucket.
func TestRateLimitUserBucket(t *testing.T) {
	rl := NewRateLimiter()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < int(userLimit); i++ {
		rl.Allow(userKey("user-9"), userLimit, userRefillRate)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.RemoteAddr = "203.0.113.7:4567"
	req = req.WithContext(context.WithValue(req.Context(), userIDKey, "user-9"))
	rec := httptest.NewRecorder()
	RateLimit(rl)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Limit"); got != "60" {
		t.Errorf("X-RateLimit-Limit = %q, want 60", got)
	}
}

// TestRateLimiterStatus verifies Status reports the bucket's limit, floored
// remaining tokens, and a Unix reset timestamp; and returns false before the
// bucket exists.
func TestRateLimiterStatus(t *testing.T) {
	rl := NewRateLimiter()

	if _, ok := rl.Status("ip:1.2.3.4"); ok {
		t.Error("Status() before Allow = ok, want !ok")
	}

	rl.Allow("ip:1.2.3.4", 100, ipRefillRate)

	st, ok := rl.Status("ip:1.2.3.4")
	if !ok {
		t.Fatal("Status() after Allow = !ok, want ok")
	}
	if st.Limit != 100 {
		t.Errorf("Limit = %v, want 100", st.Limit)
	}
	if st.Remaining != 99 {
		t.Errorf("Remaining = %v, want 99", st.Remaining)
	}
	if st.Reset == 0 {
		t.Error("Reset = 0, want a positive Unix timestamp")
	}
}

// TestRateLimiterUsage verifies Usage reports the ip and user buckets, filling
// full-capacity defaults for buckets that have not been created yet.
func TestRateLimiterUsage(t *testing.T) {
	rl := NewRateLimiter()

	// Drain one token from the user bucket so its remaining differs from the
	// default full value.
	rl.Allow(userKey("user-9"), userLimit, userRefillRate)

	usage := rl.Usage("203.0.113.7", "user-9")

	ip, ok := usage["ip"]
	if !ok {
		t.Fatal(`usage missing "ip" key`)
	}
	if ip.Limit != ipLimit || ip.Remaining != ipLimit {
		t.Errorf("ip = %+v, want full default (limit %v, remaining %v)", ip, ipLimit, ipLimit)
	}

	user, ok := usage["user"]
	if !ok {
		t.Fatal(`usage missing "user" key`)
	}
	if user.Limit != userLimit {
		t.Errorf("user.Limit = %v, want %v", user.Limit, userLimit)
	}
	if user.Remaining != userLimit-1 {
		t.Errorf("user.Remaining = %v, want %v", user.Remaining, userLimit-1)
	}
}
