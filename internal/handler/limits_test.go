package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SalvucciFacundo/api-gateway/internal/middleware"
)

// limitsResponse mirrors the JSON shape returned by GET /api/v1/limits/status.
type limitsResponse struct {
	IP   middleware.BucketStatus `json:"ip"`
	User middleware.BucketStatus `json:"user"`
}

// TestLimitsStatusHandler verifies RATE-005: the endpoint returns 200 with
// JSON describing the authenticated user's ip and user buckets. Fresh buckets
// report their documented defaults (IP 100/min, user 60/min).
func TestLimitsStatusHandler(t *testing.T) {
	rl := middleware.NewRateLimiter()
	h := LimitsStatusHandler(rl)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits/status", nil)
	req.RemoteAddr = "203.0.113.7:4567"
	req = req.WithContext(middleware.ContextWithUserID(req.Context(), "user-9"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var resp limitsResponse
	decodeJSON(t, rec, &resp)

	if resp.IP.Limit != 100 {
		t.Errorf("ip.limit = %v, want 100", resp.IP.Limit)
	}
	if resp.IP.Remaining != 100 {
		t.Errorf("ip.remaining = %v, want 100 (fresh bucket)", resp.IP.Remaining)
	}
	if resp.User.Limit != 60 {
		t.Errorf("user.limit = %v, want 60", resp.User.Limit)
	}
	if resp.User.Remaining != 60 {
		t.Errorf("user.remaining = %v, want 60 (fresh bucket)", resp.User.Remaining)
	}
	if resp.IP.Reset == 0 || resp.User.Reset == 0 {
		t.Error("reset timestamps are zero, want positive Unix timestamps")
	}
}
