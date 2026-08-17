package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret-key-for-unit-tests")

// accessClaims builds a valid access-typed Claims set for the given user, with
// an optional expiry override via the duration argument.
func accessClaims(userID string, lifetime time.Duration) *model.Claims {
	now := time.Now()
	return &model.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    model.Issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Type: "access",
	}
}

// signToken signs claims with secret and returns the compact token string.
func signToken(t *testing.T, secret []byte, claims *model.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}

// signNoneToken signs a token with the "none" algorithm to exercise the
// algorithm-confusion defense.
func signNoneToken(t *testing.T, userID string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodNone, accessClaims(userID, model.AccessExpiration))
	signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("SignedString(none) error = %v", err)
	}
	return signed
}

// TestAuthenticate verifies JWT-AUTH-004: only a correctly signed, unexpired,
// correctly-issued access token authenticates.
func TestAuthenticate(t *testing.T) {
	validToken := signToken(t, testSecret, accessClaims("user-123", model.AccessExpiration))

	refreshToken := signToken(t, testSecret, func() *model.Claims {
		c := accessClaims("user-123", model.RefreshExpiration)
		c.Type = "refresh"
		return c
	}())

	wrongIssuer := signToken(t, testSecret, func() *model.Claims {
		c := accessClaims("user-123", model.AccessExpiration)
		c.Issuer = "some-other-issuer"
		return c
	}())

	tests := []struct {
		name       string
		header     string
		wantOK     bool
		wantUserID string
	}{
		{name: "valid access token", header: "Bearer " + validToken, wantOK: true, wantUserID: "user-123"},
		{name: "missing header", header: "", wantOK: false},
		{name: "non-bearer scheme", header: "Basic abc", wantOK: false},
		{name: "empty token", header: "Bearer ", wantOK: false},
		{name: "malformed token", header: "Bearer not-a-jwt", wantOK: false},
		{name: "wrong secret", header: "Bearer " + signToken(t, []byte("some-other-secret"), accessClaims("user-123", model.AccessExpiration)), wantOK: false},
		{name: "expired token", header: "Bearer " + signToken(t, testSecret, accessClaims("user-123", -time.Minute)), wantOK: false},
		{name: "wrong issuer", header: "Bearer " + wrongIssuer, wantOK: false},
		{name: "refresh token rejected", header: "Bearer " + refreshToken, wantOK: false},
		{name: "empty subject rejected", header: "Bearer " + signToken(t, testSecret, accessClaims("", model.AccessExpiration)), wantOK: false},
		{name: "none algorithm rejected", header: "Bearer " + signNoneToken(t, "user-123"), wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, ok := authenticate(tt.header, testSecret)
			if ok != tt.wantOK {
				t.Fatalf("authenticate() ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if claims.Subject != tt.wantUserID {
				t.Errorf("claims.Subject = %q, want %q", claims.Subject, tt.wantUserID)
			}
		})
	}
}

// TestAuthMiddleware verifies JWT-AUTH-005: the middleware grants protected
// requests only to a valid access token and sets the subject in context.
func TestAuthMiddleware(t *testing.T) {
	validToken := signToken(t, testSecret, accessClaims("user-123", model.AccessExpiration))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(userID))
	})

	t.Run("valid token reaches handler with user id in context", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		rec := httptest.NewRecorder()

		Auth(testSecret)(next).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if rec.Body.String() != "user-123" {
			t.Errorf("body = %q, want user-123", rec.Body.String())
		}
	})

	t.Run("missing token returns 401 and skips handler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		rec := httptest.NewRecorder()

		Auth(testSecret)(next).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "unauthorized") {
			t.Errorf("body = %q, want to contain unauthorized", rec.Body.String())
		}
		if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
			t.Errorf("WWW-Authenticate = %q, want Bearer", got)
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+signToken(t, []byte("wrong"), accessClaims("user-123", model.AccessExpiration)))
		rec := httptest.NewRecorder()

		Auth(testSecret)(next).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("refresh token rejected with 401", func(t *testing.T) {
		refreshToken := signToken(t, testSecret, func() *model.Claims {
			c := accessClaims("user-123", model.RefreshExpiration)
			c.Type = "refresh"
			return c
		}())

		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+refreshToken)
		rec := httptest.NewRecorder()

		Auth(testSecret)(next).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", rec.Code)
		}
	})
}
