package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SalvucciFacundo/api-gateway/internal/middleware"
	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/SalvucciFacundo/api-gateway/internal/service"
	"github.com/SalvucciFacundo/api-gateway/internal/store"
)

// getRequest runs GetCurrentUserHandler against a request carrying the given
// user ID in context, mimicking the Auth middleware's context injection.
func getRequest(t *testing.T, h http.Handler, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	if userID != "" {
		req = req.WithContext(middleware.ContextWithUserID(req.Context(), userID))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestGetCurrentUserHandlerSuccess verifies JWT-AUTH-005: an authenticated
// request returns 200 with {id, email} for the subject.
func TestGetCurrentUserHandlerSuccess(t *testing.T) {
	s := store.NewMemoryStore()
	user := seedUser(t, s, "user@example.com", "securePass123")
	svc := service.NewAuthService(s, testSecret)
	h := GetCurrentUserHandler(svc)

	rec := getRequest(t, h, user.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp model.UserResponse
	decodeJSON(t, rec, &resp)
	if resp.ID != user.ID {
		t.Errorf("id = %q, want %q", resp.ID, user.ID)
	}
	if resp.Email != "user@example.com" {
		t.Errorf("email = %q, want user@example.com", resp.Email)
	}
}

// TestGetCurrentUserHandlerUnauthenticated verifies JWT-AUTH-005: a request
// without a user ID in context returns 401.
func TestGetCurrentUserHandlerUnauthenticated(t *testing.T) {
	s := store.NewMemoryStore()
	svc := service.NewAuthService(s, testSecret)
	h := GetCurrentUserHandler(svc)

	rec := getRequest(t, h, "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body %q)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["error"] != "unauthorized" {
		t.Errorf("error = %q, want unauthorized", resp["error"])
	}
}

// TestGetCurrentUserHandlerNotFound verifies that an authenticated user ID with
// no matching store record returns 404.
func TestGetCurrentUserHandlerNotFound(t *testing.T) {
	s := store.NewMemoryStore()
	svc := service.NewAuthService(s, testSecret)
	h := GetCurrentUserHandler(svc)

	rec := getRequest(t, h, "missing-id")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %q)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["error"] != "user not found" {
		t.Errorf("error = %q, want user not found", resp["error"])
	}
}
