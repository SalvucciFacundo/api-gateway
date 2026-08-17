package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/SalvucciFacundo/api-gateway/internal/service"
	"github.com/SalvucciFacundo/api-gateway/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// TestRegisterHandlerSuccess verifies JWT-AUTH-001: a valid registration
// returns 201 with {id, email} and stores a bcrypt hash, never the plaintext.
func TestRegisterHandlerSuccess(t *testing.T) {
	s := store.NewMemoryStore()
	svc := service.NewAuthService(s, testSecret)
	h := RegisterHandler(svc)

	rec := doRequest(t, h, http.MethodPost, "/api/v1/auth/register",
		`{"email":"user@example.com","password":"securePass123"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var resp model.UserResponse
	decodeJSON(t, rec, &resp)
	if resp.ID == "" {
		t.Error("id empty, want a generated UUID")
	}
	if resp.Email != "user@example.com" {
		t.Errorf("email = %q, want user@example.com", resp.Email)
	}

	stored, err := s.GetByEmail(t.Context(), "user@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if stored.PasswordHash == "securePass123" {
		t.Error("stored password is plaintext, want a bcrypt hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("securePass123")); err != nil {
		t.Errorf("stored hash does not match password: %v", err)
	}
}

// TestRegisterHandlerErrors verifies registration rejects malformed bodies,
// invalid emails, weak passwords (400) and duplicate emails (409).
func TestRegisterHandlerErrors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantErr    string
	}{
		{name: "malformed JSON", body: `{`, wantStatus: http.StatusBadRequest, wantErr: "invalid request body"},
		{name: "invalid email", body: `{"email":"nope","password":"securePass123"}`, wantStatus: http.StatusBadRequest, wantErr: "invalid email"},
		{name: "weak password", body: `{"email":"user@example.com","password":"short"}`, wantStatus: http.StatusBadRequest, wantErr: "weak password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemoryStore()
			svc := service.NewAuthService(s, testSecret)
			h := RegisterHandler(svc)

			rec := doRequest(t, h, http.MethodPost, "/api/v1/auth/register", tt.body)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %q)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var resp map[string]string
			decodeJSON(t, rec, &resp)
			if resp["error"] != tt.wantErr {
				t.Errorf("error = %q, want %q", resp["error"], tt.wantErr)
			}
		})
	}
}

// TestRegisterHandlerDuplicate verifies JWT-AUTH-001's duplicate case: a second
// registration with the same email returns 409 with an error message.
func TestRegisterHandlerDuplicate(t *testing.T) {
	s := store.NewMemoryStore()
	svc := service.NewAuthService(s, testSecret)
	h := RegisterHandler(svc)

	first := doRequest(t, h, http.MethodPost, "/api/v1/auth/register",
		`{"email":"dup@example.com","password":"securePass123"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first registration status = %d, want 201", first.Code)
	}

	second := doRequest(t, h, http.MethodPost, "/api/v1/auth/register",
		`{"email":"dup@example.com","password":"securePass123"}`)
	if second.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body %q)", second.Code, second.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, second, &resp)
	if resp["error"] == "" {
		t.Error("error message empty, want a non-empty conflict message")
	}
}

// TestLoginHandlerSuccess verifies JWT-AUTH-002: valid credentials return 200
// with non-empty access and refresh tokens.
func TestLoginHandlerSuccess(t *testing.T) {
	s := store.NewMemoryStore()
	seedUser(t, s, "user@example.com", "securePass123")
	svc := service.NewAuthService(s, testSecret)
	h := LoginHandler(svc)

	rec := doRequest(t, h, http.MethodPost, "/api/v1/auth/login",
		`{"email":"user@example.com","password":"securePass123"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	decodeJSON(t, rec, &resp)
	if resp.AccessToken == "" {
		t.Error("access_token empty, want a JWT")
	}
	if resp.RefreshToken == "" {
		t.Error("refresh_token empty, want a JWT")
	}
}

// TestLoginHandlerInvalidCredentials verifies JWT-AUTH-002: wrong credentials
// return 401 with a generic message (no field leakage).
func TestLoginHandlerInvalidCredentials(t *testing.T) {
	s := store.NewMemoryStore()
	seedUser(t, s, "user@example.com", "securePass123")
	svc := service.NewAuthService(s, testSecret)
	h := LoginHandler(svc)

	rec := doRequest(t, h, http.MethodPost, "/api/v1/auth/login",
		`{"email":"user@example.com","password":"wrongPassword"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %q)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["error"] != "invalid credentials" {
		t.Errorf("error = %q, want invalid credentials", resp["error"])
	}
}

// TestRefreshHandlerSuccess verifies JWT-AUTH-003: a valid refresh token
// returns 200 with a fresh token pair.
func TestRefreshHandlerSuccess(t *testing.T) {
	s := store.NewMemoryStore()
	seedUser(t, s, "user@example.com", "securePass123")
	svc := service.NewAuthService(s, testSecret)

	login := doRequest(t, LoginHandler(svc), http.MethodPost, "/api/v1/auth/login",
		`{"email":"user@example.com","password":"securePass123"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200", login.Code)
	}
	var loginResp model.AuthResponse
	decodeJSON(t, login, &loginResp)

	body, err := json.Marshal(model.RefreshRequest{RefreshToken: loginResp.RefreshToken})
	if err != nil {
		t.Fatalf("marshal refresh request: %v", err)
	}
	rec := doRequest(t, RefreshHandler(svc), http.MethodPost, "/api/v1/auth/refresh", string(body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	decodeJSON(t, rec, &resp)
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("refresh returned empty tokens: %+v", resp)
	}
}

// TestRefreshHandlerInvalidToken verifies JWT-AUTH-003: an invalid refresh
// token returns 401.
func TestRefreshHandlerInvalidToken(t *testing.T) {
	s := store.NewMemoryStore()
	svc := service.NewAuthService(s, testSecret)
	h := RefreshHandler(svc)

	rec := doRequest(t, h, http.MethodPost, "/api/v1/auth/refresh",
		`{"refresh_token":"not-a-jwt"}`)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %q)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["error"] != "invalid token" {
		t.Errorf("error = %q, want invalid token", resp["error"])
	}
}
