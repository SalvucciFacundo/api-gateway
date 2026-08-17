package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newJSONLogger returns a logger writing JSON lines to buf, mirroring the
// production JSON handler (LOG-001).
func newJSONLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

// parseLog decodes the single JSON line in buf and fails the test if it is not
// valid JSON.
func parseLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("log output is not valid JSON: %v\n%s", err, buf.String())
	}
	return m
}

// TestLoggingEmitsStructuredRecord verifies LOG-004: every request is logged
// with method, path, status, duration_ms and request_id as structured fields.
func TestLoggingEmitsStructuredRecord(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	chain := RequestID(Logging(logger)(handler))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	m := parseLog(t, &buf)

	if got := m["msg"]; got != "request" {
		t.Errorf("log msg = %v, want %q", got, "request")
	}
	if got := m["level"]; got != "INFO" {
		t.Errorf("log level = %v, want %q", got, "INFO")
	}
	if got := m["method"]; got != http.MethodGet {
		t.Errorf("log method = %v, want %q", got, http.MethodGet)
	}
	if got := m["path"]; got != "/api/v1/users/me" {
		t.Errorf("log path = %v, want %q", got, "/api/v1/users/me")
	}
	if got := m["status"]; got != float64(http.StatusOK) {
		t.Errorf("log status = %v, want %v", got, float64(http.StatusOK))
	}
	if requestID, _ := m["request_id"].(string); requestID == "" {
		t.Error("log request_id is empty, want a generated UUID")
	}
	durationMS, ok := m["duration_ms"].(float64)
	if !ok {
		t.Fatalf("log duration_ms = %v (%T), want a number", m["duration_ms"], m["duration_ms"])
	}
	if durationMS < 0 {
		t.Errorf("log duration_ms = %v, want non-negative", durationMS)
	}
	if durationMS != float64(int64(durationMS)) {
		t.Errorf("log duration_ms = %v, want an integer number of milliseconds", durationMS)
	}
}

// TestLoggingCapturesStatus verifies the middleware records the actual status
// code the handler wrote, not a hardcoded 200.
func TestLoggingCapturesStatus(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var buf bytes.Buffer
			logger := newJSONLogger(&buf)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			})

			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			Logging(logger)(handler).ServeHTTP(rec, req)

			m := parseLog(t, &buf)
			if got := m["status"]; got != float64(code) {
				t.Errorf("log status = %v, want %v", got, float64(code))
			}
		})
	}
}

// TestLoggingWithoutRequestIDDoesNotPanic verifies the middleware degrades to
// an empty request_id when no RequestID middleware ran first, rather than
// panicking on a missing context value.
func TestLoggingWithoutRequestIDDoesNotPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := newJSONLogger(&buf)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	Logging(logger)(handler).ServeHTTP(rec, req)

	m := parseLog(t, &buf)
	if requestID, _ := m["request_id"].(string); requestID != "" {
		t.Errorf("log request_id = %q, want empty string when no RequestID ran", requestID)
	}
}
