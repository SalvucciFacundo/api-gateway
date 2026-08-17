package handler

import (
	"net/http"

	"github.com/SalvucciFacundo/api-gateway/internal/middleware"
)

// LimitsStatusHandler handles GET /api/v1/limits/status (RATE-005). It reports
// the authenticated user's per-IP and per-user rate limit buckets as JSON.
func LimitsStatusHandler(limiter *middleware.RateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := middleware.UserIDFromContext(r.Context())
		ip := middleware.ClientIP(r)

		writeJSON(w, http.StatusOK, limiter.Usage(ip, userID))
	}
}
