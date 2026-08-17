package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/SalvucciFacundo/api-gateway/internal/store"
)

// unavailableStore is a Store whose Ping always fails, simulating a down
// backing store for the readiness probe.
type unavailableStore struct {
	store.Store
}

// Ping reports the store as unavailable.
func (unavailableStore) Ping(context.Context) error {
	return errors.New("store unavailable")
}

// TestLivenessHandler verifies HEALTH-001: the liveness probe always returns
// 200 with {status: "ok"}.
func TestLivenessHandler(t *testing.T) {
	h := LivenessHandler()

	rec := doRequest(t, h, http.MethodGet, "/healthz", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
}

// TestReadinessHandlerReady verifies HEALTH-003: a healthy store yields 200
// with {status: "ready"}.
func TestReadinessHandlerReady(t *testing.T) {
	h := ReadinessHandler(store.NewMemoryStore())

	rec := doRequest(t, h, http.MethodGet, "/readyz", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ready" {
		t.Errorf("status = %q, want ready", resp["status"])
	}
}

// TestReadinessHandlerNotReady verifies HEALTH-003: an unavailable store (or a
// nil/uninitialized store) yields 503 with {status: "not ready"}.
func TestReadinessHandlerNotReady(t *testing.T) {
	t.Run("failing store", func(t *testing.T) {
		h := ReadinessHandler(unavailableStore{})
		rec := doRequest(t, h, http.MethodGet, "/readyz", "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (body %q)", rec.Code, rec.Body.String())
		}
		var resp map[string]string
		decodeJSON(t, rec, &resp)
		if resp["status"] != "not ready" {
			t.Errorf("status = %q, want not ready", resp["status"])
		}
	})

	t.Run("nil store", func(t *testing.T) {
		h := ReadinessHandler(nil)
		rec := doRequest(t, h, http.MethodGet, "/readyz", "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503 (body %q)", rec.Code, rec.Body.String())
		}
	})
}
