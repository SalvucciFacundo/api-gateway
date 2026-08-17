package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/SalvucciFacundo/api-gateway/internal/store"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// testSecret is the JWT signing secret shared by handler tests. It is only
// ever used in tests, never in production.
var testSecret = []byte("test-secret-key-for-unit-tests")

// seedUser inserts a user with a bcrypt.MinCost hash directly into the store,
// bypassing the AuthService's cost-12 hashing so handler tests stay fast.
func seedUser(t *testing.T, s store.Store, email, password string) *model.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	u := &model.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}
	if err := s.Create(context.Background(), u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	return u
}

// doRequest runs h against a request with the given method, target and raw body.
func doRequest(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rd)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// decodeJSON unmarshals the response body into v, failing the test on error.
func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder, v *T) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
}
