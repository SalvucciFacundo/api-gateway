package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
)

// testSecret is the JWT signing secret shared by server tests. It is only ever
// used in tests, never in production.
var testSecret = []byte("test-secret-key-for-unit-tests")

// testStaticFS is a minimal embedded frontend build for the SPA fallback tests.
var testStaticFS = fstest.MapFS{
	"index.html": &fstest.MapFile{Data: []byte("<!doctype html><html><body>SPA</body></html>")},
}

// discardLogger returns a slog JSON logger that writes nowhere, so tests stay
// quiet while still exercising the JSON handler (LOG-001).
func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// newTestServer builds a Server wired with a discard logger and a test static
// FS, exercising the full middleware chain and route set.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(Config{
		Port:      "8080",
		JWTSecret: testSecret,
		Logger:    discardLogger(),
		StaticFS:  testStaticFS,
	})
}

// do performs a request against h and returns the recorded response.
func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
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

// registerAndLogin runs the full register → login flow against h and returns
// the access token, or fails the test.
func registerAndLogin(t *testing.T, h http.Handler, email, password string) string {
	t.Helper()

	rec := do(t, h, http.MethodPost, "/api/v1/auth/register",
		`{"email":"`+email+`","password":"`+password+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}

	rec = do(t, h, http.MethodPost, "/api/v1/auth/login",
		`{"email":"`+email+`","password":"`+password+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp model.AuthResponse
	decodeJSON(t, rec, &resp)
	if resp.AccessToken == "" {
		t.Fatal("access_token empty, want a JWT")
	}
	return resp.AccessToken
}

// TestServerLiveness verifies HEALTH-001/004: GET /healthz returns 200 with
// {status:"ok"} without authentication.
func TestServerLiveness(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv.Handler(), http.MethodGet, "/healthz", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %q, want ok", resp["status"])
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header missing, want a request ID")
	}
}

// TestServerReadiness verifies HEALTH-002/003: GET /readyz returns 200 with
// {status:"ready"} when the store is healthy.
func TestServerReadiness(t *testing.T) {
	srv := newTestServer(t)

	rec := do(t, srv.Handler(), http.MethodGet, "/readyz", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["status"] != "ready" {
		t.Errorf("status = %q, want ready", resp["status"])
	}
}

// TestServerMetricsEndpoint verifies METRICS-003/004: GET /metrics returns 200
// with Prometheus text format, unauthenticated, and exposes the request counter
// after at least one request has been served.
func TestServerMetricsEndpoint(t *testing.T) {
	srv := newTestServer(t)

	// Generate at least one request so the counter has a sample.
	do(t, srv.Handler(), http.MethodGet, "/healthz", "")

	rec := do(t, srv.Handler(), http.MethodGet, "/metrics", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "http_requests_total") {
		t.Errorf("body missing http_requests_total, got:\n%s", body)
	}
	if !strings.Contains(body, "http_request_duration_seconds") {
		t.Errorf("body missing http_request_duration_seconds, got:\n%s", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want a Prometheus text/plain type", ct)
	}
}

// TestIntegrationAuthFlow verifies the full JWT-AUTH-001/002/005 journey:
// register → login → authenticated GET /api/v1/users/me.
func TestIntegrationAuthFlow(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()

	rec := do(t, h, http.MethodPost, "/api/v1/auth/register",
		`{"email":"flow@example.com","password":"securePass123"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201 (body %q)", rec.Code, rec.Body.String())
	}
	var user model.UserResponse
	decodeJSON(t, rec, &user)
	if user.Email != "flow@example.com" {
		t.Errorf("registered email = %q, want flow@example.com", user.Email)
	}

	rec = do(t, h, http.MethodPost, "/api/v1/auth/login",
		`{"email":"flow@example.com","password":"securePass123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var auth model.AuthResponse
	decodeJSON(t, rec, &auth)
	if auth.AccessToken == "" || auth.RefreshToken == "" {
		t.Fatalf("login returned empty tokens: %+v", auth)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	me := httptest.NewRecorder()
	h.ServeHTTP(me, req)
	if me.Code != http.StatusOK {
		t.Fatalf("users/me status = %d, want 200 (body %q)", me.Code, me.Body.String())
	}
	var meResp model.UserResponse
	decodeJSON(t, me, &meResp)
	if meResp.Email != "flow@example.com" {
		t.Errorf("users/me email = %q, want flow@example.com", meResp.Email)
	}
	if meResp.ID != user.ID {
		t.Errorf("users/me id = %q, want %q", meResp.ID, user.ID)
	}
}

// TestIntegrationProtectedRequiresAuth verifies JWT-AUTH-005: a protected
// endpoint without a token (or with an invalid one) returns 401.
func TestIntegrationProtectedRequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()

	t.Run("missing token", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/api/v1/users/me", "")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 (body %q)", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer not-a-jwt")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401 (body %q)", rec.Code, rec.Body.String())
		}
	})
}

// TestIntegrationRateLimitLogin429 verifies RATE-003/006: after 10 login
// attempts the 11th returns 429 with a Retry-After header.
func TestIntegrationRateLimitLogin429(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()

	// The login endpoint has a 10/min anti-brute-force bucket. A non-existent
	// email short-circuits the handler (no bcrypt) so each attempt is cheap.
	for i := 0; i < 10; i++ {
		rec := do(t, h, http.MethodPost, "/api/v1/auth/login",
			`{"email":"missing@example.com","password":"whatever1"}`)
		if rec.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d already rate limited (want the first 10 to pass through)", i+1)
		}
	}

	rec := do(t, h, http.MethodPost, "/api/v1/auth/login",
		`{"email":"missing@example.com","password":"whatever1"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (body %q)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header missing, want a retry delay")
	}
	var resp map[string]string
	decodeJSON(t, rec, &resp)
	if resp["error"] != "rate limit exceeded" {
		t.Errorf("error = %q, want rate limit exceeded", resp["error"])
	}
}

// TestServerLimitsStatus verifies RATE-005: an authenticated request to
// /api/v1/limits/status returns per-ip and per-user bucket JSON.
func TestServerLimitsStatus(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()

	token := registerAndLogin(t, h, "limits@example.com", "securePass123")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/limits/status", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	var resp struct {
		IP   map[string]float64 `json:"ip"`
		User map[string]float64 `json:"user"`
	}
	decodeJSON(t, rec, &resp)
	if resp.IP["limit"] != 100 {
		t.Errorf("ip.limit = %v, want 100", resp.IP["limit"])
	}
	if resp.User["limit"] != 60 {
		t.Errorf("user.limit = %v, want 60", resp.User["limit"])
	}
}

// TestServerSPAFallback verifies FE-003: a non-API client route serves
// index.html, while an unknown API route does not (it 404s instead).
func TestServerSPAFallback(t *testing.T) {
	srv := newTestServer(t)
	h := srv.Handler()

	t.Run("client route", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/dashboard", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "SPA") {
			t.Errorf("body = %q, want index.html content", rec.Body.String())
		}
	})

	t.Run("unknown API route", func(t *testing.T) {
		rec := do(t, h, http.MethodGet, "/api/v1/unknown", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body %q)", rec.Code, rec.Body.String())
		}
	})
}

// TestServerTimeouts verifies the http.Server is configured with the designed
// read/write/idle timeouts.
func TestServerTimeouts(t *testing.T) {
	srv := newTestServer(t)

	hs := srv.httpServer
	if hs.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %v, want 15s", hs.ReadTimeout)
	}
	if hs.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 5s", hs.ReadHeaderTimeout)
	}
	if hs.WriteTimeout != 15*time.Second {
		t.Errorf("WriteTimeout = %v, want 15s", hs.WriteTimeout)
	}
	if hs.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", hs.IdleTimeout)
	}
}

// TestServerGracefulShutdown verifies Start/Shutdown behavior: the server
// serves requests until the context is cancelled, then shuts down cleanly.
func TestServerGracefulShutdown(t *testing.T) {
	srv := newTestServer(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.serve(ctx, ln) }()

	baseURL := "http://" + ln.Addr().String()
	var resp *http.Response
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err = http.Get(baseURL + "/healthz")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not come up: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("serve returned error after shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down within 5s")
	}
}

// TestServerStartGracefulShutdown verifies the public Start entrypoint: it
// binds its configured address, then returns nil once ctx is cancelled.
func TestServerStartGracefulShutdown(t *testing.T) {
	srv := NewServer(Config{Port: "0", JWTSecret: testSecret, Logger: discardLogger()})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start(ctx) }()

	// Allow the server to bind and begin serving before cancelling.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after ctx cancellation")
	}
}

// TestNewServerDefaults verifies Config zero values resolve to sane defaults:
// a fresh in-memory store, a non-nil JSON logger, the default listen address,
// and no SPA route when no static FS is provided.
func TestNewServerDefaults(t *testing.T) {
	srv := NewServer(Config{})

	if srv.store == nil {
		t.Error("store is nil, want a fresh in-memory store")
	}
	if srv.logger == nil {
		t.Error("logger is nil, want a JSON discard logger")
	}
	if srv.httpServer.Addr != ":8080" {
		t.Errorf("httpServer.Addr = %q, want :8080", srv.httpServer.Addr)
	}
	if srv.Handler() == nil {
		t.Error("Handler() returned nil, want a router")
	}

	// Without a static FS the SPA fallback must not be mounted.
	rec := do(t, srv.Handler(), http.MethodGet, "/dashboard", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (no static FS)", rec.Code)
	}
}
