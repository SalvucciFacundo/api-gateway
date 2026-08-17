// Package middleware holds the HTTP middleware for the gateway.
package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

// contextKey is an unexported key type so context values cannot collide with
// other packages' keys.
type contextKey string

// userIDKey is the context key under which Auth stores the authenticated user ID.
const userIDKey contextKey = "userID"

// bearerScheme is the auth scheme name used in both the Authorization header
// prefix and the WWW-Authenticate challenge.
const bearerScheme = "Bearer"

// bearerPrefix is the expected Authorization header scheme prefix.
const bearerPrefix = bearerScheme + " "

// Auth returns middleware that validates the Bearer access token in the
// Authorization header and stores the authenticated subject (user ID) in the
// request context. Requests without a valid access token receive 401 and are
// never passed to the wrapped handler.
func Auth(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := authenticate(r.Header.Get("Authorization"), jwtSecret)
			if !ok {
				writeUnauthorized(w)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticate validates the Bearer token in authHeader. It returns the token
// claims when the token is a correctly signed, unexpired, correctly-issued
// access token; otherwise it returns false.
func authenticate(authHeader string, jwtSecret []byte) (*model.Claims, bool) {
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return nil, false
	}
	tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, bearerPrefix))
	if tokenString == "" {
		return nil, false
	}

	claims := &model.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, false
	}
	if claims.Issuer != model.Issuer {
		return nil, false
	}
	if claims.Type != "access" {
		return nil, false
	}
	if claims.Subject == "" {
		return nil, false
	}

	return claims, true
}

// UserIDFromContext returns the authenticated user ID stored by Auth, or false
// if no user ID is present in the context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}

// ContextWithUserID returns a context carrying the given user ID under the same
// key Auth stores it. It is the inverse of UserIDFromContext and lets in-process
// callers (notably handler tests) construct an authenticated context without
// going through the HTTP layer.
func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// writeUnauthorized emits a 401 with a JSON error body and a Bearer
// WWW-Authenticate challenge header.
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", bearerScheme)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}
