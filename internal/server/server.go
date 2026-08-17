// Package server composes the gateway's HTTP stack: the router with its
// middleware chain, the handlers, and the http.Server that serves them. It is
// the single place where requestID → logging → metrics → auth → rateLimit →
// handler ordering is expressed.
package server

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/handler"
	"github.com/SalvucciFacundo/api-gateway/internal/middleware"
	"github.com/SalvucciFacundo/api-gateway/internal/service"
	"github.com/SalvucciFacundo/api-gateway/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server-level timeouts. Values follow the design's hardening section: bounded
// read/write windows so slow clients cannot hold connections open forever.
const (
	readTimeout       = 15 * time.Second
	readHeaderTimeout = 5 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 30 * time.Second

	// cleanupMaxAge is how long a rate limit bucket is kept after its last use
	// before the cleanup goroutine drops it.
	cleanupMaxAge = 10 * time.Minute
)

// defaultPort is used when Config.Port is empty.
const defaultPort = "8080"

// Config configures NewServer. JWTSecret is required in production (main fails
// fast without it); it may be empty in tests that make no authenticated
// request. Logger, StaticFS and Store are optional with sensible defaults.
type Config struct {
	Port      string       // listen port; defaults to "8080"
	JWTSecret []byte       // HS256 signing secret for access tokens
	Logger    *slog.Logger // optional; nil → JSON handler writing to io.Discard
	StaticFS  fs.FS        // optional embedded SPA assets; nil disables the SPA fallback
	Store     store.Store  // optional; nil → a fresh in-memory store
}

// Server owns the gateway's HTTP server, router, and its collaborators.
type Server struct {
	httpServer  *http.Server
	router      *chi.Mux
	store       store.Store
	authService *service.AuthService
	limiter     *middleware.RateLimiter
	logger      *slog.Logger
	jwtSecret   []byte
	staticFS    fs.FS
}

// NewServer wires the store, auth service, rate limiter, router and http.Server
// into a ready-to-serve Server.
func NewServer(cfg Config) *Server {
	st := cfg.Store
	if st == nil {
		st = store.NewMemoryStore()
	}

	logger := cfg.Logger
	if logger == nil {
		// LOG-001: structured JSON output is the default even when unobserved.
		logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
	}

	s := &Server{
		store:       st,
		authService: service.NewAuthService(st, cfg.JWTSecret),
		limiter:     middleware.NewRateLimiter(),
		logger:      logger,
		jwtSecret:   cfg.JWTSecret,
		staticFS:    cfg.StaticFS,
	}
	s.router = s.buildRouter()

	port := cfg.Port
	if port == "" {
		port = defaultPort
	}
	s.httpServer = &http.Server{
		Addr:              ":" + port,
		Handler:           s.router,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	return s
}

// buildRouter assembles the chi router with the global middleware chain and the
// full route set. Order matters: requestID runs first so every downstream
// middleware and handler can read the ID, then logging and metrics wrap the
// rest; auth and rate limiting apply only to their route groups.
func (s *Server) buildRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logging(s.logger))
	r.Use(middleware.Metrics())

	// Public probes and metrics (HEALTH-004, METRICS-004): no auth.
	r.Get("/healthz", handler.LivenessHandler())
	r.Get("/readyz", handler.ReadinessHandler(s.store))
	r.Handle("/metrics", promhttp.Handler())

	// Auth routes: no auth middleware, but rate limited (IP + login endpoint).
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(middleware.RateLimit(s.limiter))
		r.Post("/register", handler.RegisterHandler(s.authService))
		r.Post("/login", handler.LoginHandler(s.authService))
		r.Post("/refresh", handler.RefreshHandler(s.authService))
	})

	// Protected routes: auth then rate limiting.
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(s.jwtSecret))
		r.Use(middleware.RateLimit(s.limiter))
		r.Get("/api/v1/users/me", handler.GetCurrentUserHandler(s.authService))
		r.Get("/api/v1/limits/status", handler.LimitsStatusHandler(s.limiter))
	})

	// SPA fallback for non-API client routes (FE-003).
	if s.staticFS != nil {
		r.Get("/*", s.spaFallback())
	}

	return r
}

// spaFallback serves the embedded SPA for client-side routes. API routes that
// did not match any handler return 404 instead of index.html, so unknown API
// paths are never silently rewritten (FE-003).
func (s *Server) spaFallback() http.HandlerFunc {
	spa := handler.SPAHandler(s.staticFS)
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		spa.ServeHTTP(w, r)
	}
}

// Handler returns the router, letting tests (and embedding code) serve the
// stack without binding a network port.
func (s *Server) Handler() http.Handler {
	return s.router
}

// Start serves the router until ctx is cancelled, then shuts down gracefully.
// It also launches the rate limiter's bucket cleanup goroutine for the lifetime
// of ctx.
func (s *Server) Start(ctx context.Context) error {
	go s.limiter.StartCleanup(ctx, cleanupMaxAge)
	return s.serve(ctx, nil)
}

// serve runs the HTTP server on ln until ctx is cancelled or the server exits.
// When ln is nil, a listener is created from the server's configured address.
func (s *Server) serve(ctx context.Context, ln net.Listener) error {
	if ln == nil {
		var err error
		ln, err = net.Listen("tcp", s.httpServer.Addr)
		if err != nil {
			return err
		}
	}

	errCh := make(chan error, 1)
	go func() { errCh <- s.httpServer.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Shutdown gracefully stops the HTTP server, waiting up to ctx's deadline for
// active requests to finish.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
