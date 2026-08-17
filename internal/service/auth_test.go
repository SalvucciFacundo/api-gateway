package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/SalvucciFacundo/api-gateway/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var testSecret = []byte("test-secret-key-for-unit-tests")

// newTestService builds an AuthService backed by a fresh MemoryStore, using
// bcrypt.MinCost so registration stays fast in tests.
func newTestService(t *testing.T) *AuthService {
	t.Helper()
	svc := NewAuthService(store.NewMemoryStore(), testSecret)
	svc.bcryptCost = bcrypt.MinCost
	return svc
}

func registerUser(t *testing.T, svc *AuthService, email, password string) *model.User {
	t.Helper()
	user, err := svc.Register(context.Background(), model.RegisterRequest{Email: email, Password: password})
	if err != nil {
		t.Fatalf("Register(%q) error = %v", email, err)
	}
	return user
}

// signTestToken signs the given claims with the provided secret, failing the
// test if signing fails.
func signTestToken(t *testing.T, secret []byte, claims *model.Claims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("SignedString() error = %v", err)
	}
	return signed
}

// parseTestToken decodes and validates a token signed with testSecret,
// returning its claims. It fails the test on any parse or validation error.
func parseTestToken(t *testing.T, tokenString string) *model.Claims {
	t.Helper()
	claims := &model.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return testSecret, nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("token invalid: %v", err)
	}
	return claims
}

// TestNewAuthServiceDefaults verifies the constructor wires the store, secret,
// and a bcrypt cost of 12.
func TestNewAuthServiceDefaults(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewAuthService(s, testSecret)

	if svc.store != s {
		t.Error("store not wired into AuthService")
	}
	if len(svc.jwtSecret) == 0 {
		t.Error("jwtSecret empty, want the provided secret")
	}
	if svc.bcryptCost != 12 {
		t.Errorf("bcryptCost = %d, want 12", svc.bcryptCost)
	}
}

// TestRegisterSuccess verifies JWT-AUTH-001: a valid registration stores a user
// with a generated UUID and a bcrypt hash (never the plaintext password).
func TestRegisterSuccess(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	user, err := svc.Register(ctx, model.RegisterRequest{Email: "user@example.com", Password: "securePass123"})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.ID == "" {
		t.Error("ID empty, want a generated UUID")
	}
	if user.Email != "user@example.com" {
		t.Errorf("Email = %q, want user@example.com", user.Email)
	}
	if user.PasswordHash == "securePass123" {
		t.Error("PasswordHash is plaintext, want a bcrypt hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("securePass123")); err != nil {
		t.Errorf("password does not match stored hash: %v", err)
	}

	stored, err := svc.store.GetByEmail(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if stored.ID != user.ID {
		t.Errorf("stored user ID = %q, want %q", stored.ID, user.ID)
	}
}

// TestRegisterValidation verifies registration rejects malformed emails and
// weak passwords (fewer than 8 characters).
func TestRegisterValidation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{name: "no at sign", email: "not-an-email", password: "securePass123", wantErr: ErrInvalidEmail},
		{name: "missing domain", email: "user@", password: "securePass123", wantErr: ErrInvalidEmail},
		{name: "missing tld", email: "user@example", password: "securePass123", wantErr: ErrInvalidEmail},
		{name: "weak password", email: "user@example.com", password: "short", wantErr: ErrWeakPassword},
		{name: "empty password", email: "user@example.com", password: "", wantErr: ErrWeakPassword},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Register(ctx, model.RegisterRequest{Email: tt.email, Password: tt.password})
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Register() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestRegisterDuplicateEmail verifies JWT-AUTH-001's duplicate case: a second
// registration with the same email is rejected.
func TestRegisterDuplicateEmail(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	registerUser(t, svc, "dup@example.com", "securePass123")

	_, err := svc.Register(ctx, model.RegisterRequest{Email: "dup@example.com", Password: "securePass123"})
	if !errors.Is(err, ErrEmailExists) {
		t.Errorf("Register() duplicate error = %v, want ErrEmailExists", err)
	}
}

// TestLoginSuccess verifies JWT-AUTH-002: valid credentials return an access
// token (type "access") and a refresh token (type "refresh") for the user.
func TestLoginSuccess(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	user := registerUser(t, svc, "user@example.com", "securePass123")

	resp, err := svc.Login(ctx, model.LoginRequest{Email: "user@example.com", Password: "securePass123"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("Login() returned empty tokens: %+v", resp)
	}

	access := parseTestToken(t, resp.AccessToken)
	if access.Subject != user.ID {
		t.Errorf("access sub = %q, want %q", access.Subject, user.ID)
	}
	if access.Type != "access" {
		t.Errorf("access type = %q, want access", access.Type)
	}

	refresh := parseTestToken(t, resp.RefreshToken)
	if refresh.Subject != user.ID {
		t.Errorf("refresh sub = %q, want %q", refresh.Subject, user.ID)
	}
	if refresh.Type != "refresh" {
		t.Errorf("refresh type = %q, want refresh", refresh.Type)
	}
}

// TestLoginInvalidCredentials verifies that wrong passwords and unknown emails
// both fail with the same error so the API never leaks which field was wrong.
func TestLoginInvalidCredentials(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	registerUser(t, svc, "user@example.com", "securePass123")

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{name: "wrong password", email: "user@example.com", password: "wrongPassword"},
		{name: "unknown email", email: "missing@example.com", password: "securePass123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Login(ctx, model.LoginRequest{Email: tt.email, Password: tt.password})
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

// TestRefreshSuccess verifies JWT-AUTH-003: a valid refresh token returns a
// fresh token pair for the same subject.
func TestRefreshSuccess(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	user := registerUser(t, svc, "user@example.com", "securePass123")
	login, err := svc.Login(ctx, model.LoginRequest{Email: "user@example.com", Password: "securePass123"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	resp, err := svc.Refresh(ctx, model.RefreshRequest{RefreshToken: login.RefreshToken})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	access := parseTestToken(t, resp.AccessToken)
	if access.Subject != user.ID {
		t.Errorf("new access sub = %q, want %q", access.Subject, user.ID)
	}
	if access.Type != "access" {
		t.Errorf("new access type = %q, want access", access.Type)
	}

	refresh := parseTestToken(t, resp.RefreshToken)
	if refresh.Type != "refresh" {
		t.Errorf("new refresh type = %q, want refresh", refresh.Type)
	}
}

// TestRefreshInvalid verifies refresh rejects garbage, access tokens, expired
// tokens, wrong-secret tokens, and wrong-issuer tokens.
func TestRefreshInvalid(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	user := registerUser(t, svc, "user@example.com", "securePass123")
	login, err := svc.Login(ctx, model.LoginRequest{Email: "user@example.com", Password: "securePass123"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	baseClaims := func(issuer string) *model.Claims {
		return &model.Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    issuer,
				Subject:   user.ID,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(model.RefreshExpiration)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Type: "refresh",
		}
	}

	expired := signTestToken(t, testSecret, &model.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    model.Issuer,
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
		Type: "refresh",
	})

	tests := []struct {
		name  string
		token string
	}{
		{name: "garbage token", token: "not-a-jwt"},
		{name: "access token used as refresh", token: login.AccessToken},
		{name: "expired refresh token", token: expired},
		{name: "wrong secret", token: signTestToken(t, []byte("some-other-secret"), baseClaims(model.Issuer))},
		{name: "wrong issuer", token: signTestToken(t, testSecret, baseClaims("some-other-issuer"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Refresh(ctx, model.RefreshRequest{RefreshToken: tt.token})
			if !errors.Is(err, ErrInvalidToken) {
				t.Errorf("Refresh() error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

// TestGetUser verifies JWT-AUTH-005: an existing ID resolves to the user, a
// missing ID surfaces the store's not-found error.
func TestGetUser(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	user := registerUser(t, svc, "user@example.com", "securePass123")

	got, err := svc.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if got.Email != "user@example.com" {
		t.Errorf("GetUser().Email = %q, want user@example.com", got.Email)
	}

	if _, err := svc.GetUser(ctx, "missing-id"); !errors.Is(err, store.ErrUserNotFound) {
		t.Errorf("GetUser(missing) error = %v, want store.ErrUserNotFound", err)
	}
}

// TestGenerateTokenClaims verifies JWT-AUTH-006/008: tokens carry iss, sub,
// exp, iat, and type, with the access lifetime of exactly 15 minutes.
func TestGenerateTokenClaims(t *testing.T) {
	svc := newTestService(t)

	token, err := svc.generateToken("user-123", "access", model.AccessExpiration)
	if err != nil {
		t.Fatalf("generateToken() error = %v", err)
	}

	claims := parseTestToken(t, token)
	if claims.Issuer != model.Issuer {
		t.Errorf("iss = %q, want %q", claims.Issuer, model.Issuer)
	}
	if claims.Subject != "user-123" {
		t.Errorf("sub = %q, want user-123", claims.Subject)
	}
	if claims.Type != "access" {
		t.Errorf("type = %q, want access", claims.Type)
	}
	if claims.ExpiresAt == nil {
		t.Error("exp = nil, want set")
	}
	if claims.IssuedAt == nil {
		t.Error("iat = nil, want set")
	}
	if got := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time); got != model.AccessExpiration {
		t.Errorf("token lifetime = %v, want %v", got, model.AccessExpiration)
	}
}
