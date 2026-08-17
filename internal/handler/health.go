package handler

import (
	"net/http"

	"github.com/SalvucciFacundo/api-gateway/internal/store"
)

// LivenessHandler handles GET /healthz (HEALTH-001). It reports that the
// process is alive and accepts traffic; it performs no dependency checks and is
// not rate limited.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// ReadinessHandler handles the readiness probe (HEALTH-002/003). It pings the
// backing store and reports "ready" (200) when reachable or "not ready" (503)
// when the store is unavailable or not initialized.
func ReadinessHandler(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil || s.Ping(r.Context()) != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
