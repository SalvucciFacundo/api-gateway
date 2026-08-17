package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// TestRequestIDGeneratesUUIDv4 verifies LOG-002: a request without an
// X-Request-ID header gets a freshly generated UUID v4 stored in context and
// echoed in the response header.
func TestRequestIDGeneratesUUIDv4(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := RequestIDFromContext(r.Context())
		if !ok {
			t.Error("RequestIDFromContext() = false, want true")
			return
		}
		parsed, err := uuid.Parse(id)
		if err != nil {
			t.Errorf("generated request ID %q is not a valid UUID: %v", id, err)
			return
		}
		if parsed.Version() != 4 {
			t.Errorf("generated request ID version = %d, want 4", parsed.Version())
		}
		_, _ = w.Write([]byte(id))
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	RequestID(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	id := rec.Body.String()
	if id == "" {
		t.Fatal("generated request ID is empty")
	}
	if got := rec.Header().Get("X-Request-ID"); got != id {
		t.Errorf("X-Request-ID response header = %q, want %q", got, id)
	}
}

// TestRequestIDPropagatesExistingHeader verifies LOG-003: an incoming
// X-Request-ID is reused in context and echoed unchanged in the response.
func TestRequestIDPropagatesExistingHeader(t *testing.T) {
	const incoming = "abc-123"
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := RequestIDFromContext(r.Context())
		if !ok || id != incoming {
			t.Errorf("context request ID = %q (ok=%v), want %q", id, ok, incoming)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", incoming)
	rec := httptest.NewRecorder()

	RequestID(next).ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != incoming {
		t.Errorf("X-Request-ID response header = %q, want %q", got, incoming)
	}
}

// TestRequestIDGeneratesUniqueIDsPerRequest verifies generated IDs are unique
// across independent requests, forcing real UUID generation rather than a
// constant.
func TestRequestIDGeneratesUniqueIDsPerRequest(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, _ := RequestIDFromContext(r.Context())
			seen[id] = true
		})).ServeHTTP(rec, req)
	}
	if len(seen) != 10 {
		t.Errorf("generated %d unique IDs across 10 requests, want 10", len(seen))
	}
}

// TestRequestIDFromContextAbsent verifies the accessor reports absence when no
// request ID is present in the context.
func TestRequestIDFromContextAbsent(t *testing.T) {
	if id, ok := RequestIDFromContext(context.Background()); ok || id != "" {
		t.Errorf("RequestIDFromContext(background) = (%q, %v), want (\"\", false)", id, ok)
	}
}
