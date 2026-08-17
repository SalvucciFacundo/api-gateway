package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// requestIDKey is the context key under which RequestID stores the request ID.
// It shares the unexported contextKey type with userIDKey (defined in auth.go)
// so neither can collide with keys from other packages.
const requestIDKey contextKey = "requestID"

// RequestID returns middleware that ensures every request carries a request ID.
//
// If the incoming X-Request-ID header is present, its value is reused so a
// client-supplied trace can propagate through the gateway. Otherwise a new
// UUID v4 is generated. The ID is stored in the request context (LOG-006) and
// echoed in the X-Request-ID response header (LOG-002/003).
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}

		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID stored by RequestID, or false if
// none is present in the context.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}
