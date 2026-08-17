package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseWriter wraps an http.ResponseWriter to capture the status code the
// handler writes. It is shared by the logging and metrics middleware, which
// both need the actual status even when the handler only calls Write.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader records the first status code and forwards it to the underlying
// writer. Later calls are ignored, matching net/http's contract that only the
// first WriteHeader takes effect.
func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

// Write records an implicit 200 status before delegating, so handlers that
// only call Write still produce a correct status.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Status returns the captured status code, defaulting to 200 when the handler
// wrote a body without an explicit WriteHeader.
func (rw *responseWriter) Status() int {
	if rw.status == 0 {
		return http.StatusOK
	}
	return rw.status
}

// Unwrap exposes the underlying writer for http.ResponseController.
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Logging returns middleware that emits one structured log line per request
// once the handler completes, recording method, path, status, duration and the
// request ID set by RequestID (LOG-004).
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID, _ := RequestIDFromContext(r.Context())

			wrapped := &responseWriter{ResponseWriter: w}
			next.ServeHTTP(wrapped, r)

			logger.InfoContext(r.Context(), "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", requestID,
			)
		})
	}
}
