# Design: API Gateway — Auth + Rate Limiting + Observability

## Technical Approach

Production-ready HTTP gateway with hexagonal architecture: Chi router, JWT auth (golang-jwt/v5), token bucket rate limiting, structured logging (slog JSON), Prometheus metrics, embedded React SPA via embed.FS. Single binary, single container, zero CORS friction. Middleware chain: `requestID → logging → metrics → auth → rateLimit → handler`.

## Architecture Decisions

### Decision: Router Choice

| Option | Tradeoff | Decision |
|--------|----------|----------|
| net/http ServeMux | Zero deps, limited middleware | ❌ Rejected |
| Chi | Implements http.Handler, middleware composition, stdlib-compatible | ✅ **Chosen** |
| Gin/Echo | Feature-rich, non-stdlib signatures | ❌ Rejected |

**Rationale**: Chi provides middleware composition while remaining stdlib-compatible. No framework lock-in, testable with httptest.

### Decision: JWT Library

| Option | Tradeoff | Decision |
|--------|----------|----------|
| golang-jwt/jwt/v5 | Active maintenance, v5 API (ParseWithClaims, Valid) | ✅ **Chosen** |
| dgrijalva/jwt-go | Deprecated, security issues | ❌ Rejected |

**Rationale**: golang-jwt/v5 is the maintained fork with modern API. Claims validation via `Valid()` method, HS256 signing.

### Decision: Rate Limiting Algorithm

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Fixed window | Simple, but burst at boundaries | ❌ Rejected |
| Sliding window | Accurate, complex | ❌ Rejected |
| Token bucket | Smooth rate, simple in-memory impl | ✅ **Chosen** |

**Rationale**: Token bucket provides smooth rate limiting, easy to implement with sync.Mutex + map, documented upgrade path to Redis.

### Decision: Frontend Integration

| Option | Tradeoff | Decision |
|--------|----------|----------|
| Separate container + CORS | Flexible, CORS complexity | ❌ Rejected |
| embed.FS + SPA fallback | Single binary, same-origin, no CORS | ✅ **Chosen** |

**Rationale**: embed.FS eliminates CORS, simplifies deployment. SPA fallback serves index.html for non-API routes.

## Data Flow

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │ HTTP Request
       ▼
┌─────────────────────────────────────┐
│  requestID middleware               │
│  - Generate/propagate UUID v4       │
│  - Set X-Request-ID header          │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  logging middleware                 │
│  - slog JSON handler                │
│  - Log method/path/status/duration  │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  metrics middleware                 │
│  - Increment counter (method,path)  │
│  - Record latency histogram         │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  auth middleware (if protected)     │
│  - Extract Bearer token             │
│  - Parse + validate JWT (HS256)     │
│  - Extract sub (userID) to context  │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  ratelimit middleware               │
│  - Token bucket per IP/user/endpoint│
│  - Set X-RateLimit-* headers        │
│  - 429 + Retry-After if exceeded    │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│  Handler                            │
│  - /api/v1/auth/register            │
│  - /api/v1/auth/login               │
│  - /api/v1/auth/refresh             │
│  - /api/v1/users/me                 │
│  - /api/v1/limits/status            │
│  - /healthz                         │
│  - /metrics                         │
│  - SPA fallback (embed.FS)          │
└─────────────────────────────────────┘
```

## File Changes

### Package Structure

```
cmd/gateway/main.go                    # Entrypoint, wiring, graceful shutdown
internal/
  model/
    user.go                            # User struct (ID, Email, PasswordHash, CreatedAt)
    claims.go                          # JWT Claims struct (iss, sub, exp, iat, type)
    bucket.go                          # Bucket struct (mu, tokens, capacity, refillRate, lastRefill)
    auth.go                            # RegisterRequest, LoginRequest, AuthResponse, RefreshRequest
  store/
    store.go                           # Store interface (Create, GetByEmail, GetByID)
    memory.go                          # MemoryStore impl (sync.RWMutex, map[string]*User)
    seed.go                            # Admin seed function
  service/
    auth.go                            # AuthService (Register, Login, Refresh, GetUser)
  middleware/
    requestid.go                       # UUID v4 generation, X-Request-ID propagation
    logging.go                         # slog JSON, request logging
    metrics.go                         # Prometheus counter + histogram
    auth.go                            # JWT validation, userID extraction
    ratelimit.go                       # Token bucket, headers, 429
  handler/
    auth.go                            # Register, Login, Refresh handlers
    users.go                           # GetCurrentUser handler
    health.go                          # Healthz handler (liveness + readiness)
    limits.go                          # LimitsStatus handler
    spa.go                             # SPA fallback handler (embed.FS)
  server/
    server.go                          # http.Server composition, router setup, middleware chain
web/                                   # React SPA source (Vite)
  src/
    pages/
      LoginPage.tsx                    # Login form → /api/v1/auth/login
      RegisterPage.tsx                 # Register form → /api/v1/auth/register
      ExplorerPage.tsx                 # JWT claims + rate limit bars + health status
    components/
      RateLimitBar.tsx                 # Visual bar from /api/v1/limits/status
      HealthStatus.tsx                 # Display /healthz status
  dist/                                # Vite build output (embedded)
embed.go                               # //go:embed web/dist
Dockerfile                             # Multi-stage: node → go → alpine
docker-compose.yml                     # Port 8080, JWT_SECRET env
```

## Interfaces / Contracts

### Store Interface

```go
// internal/store/store.go
type Store interface {
    Create(ctx context.Context, user *model.User) error
    GetByEmail(ctx context.Context, email string) (*model.User, error)
    GetByID(ctx context.Context, id string) (*model.User, error)
}
```

### MemoryStore Implementation

```go
// internal/store/memory.go
type MemoryStore struct {
    mu     sync.RWMutex
    users  map[string]*model.User  // id -> user
    emails map[string]string       // email -> id (uniqueness)
}

func NewMemoryStore() *MemoryStore
func (s *MemoryStore) Create(ctx context.Context, user *model.User) error
func (s *MemoryStore) GetByEmail(ctx context.Context, email string) (*model.User, error)
func (s *MemoryStore) GetByID(ctx context.Context, id string) (*model.User, error)
```

### Auth Service

```go
// internal/service/auth.go
type AuthService struct {
    store      store.Store
    jwtSecret  []byte
    bcryptCost int  // default: 12
}

func NewAuthService(store store.Store, jwtSecret []byte) *AuthService
func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error)
func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error)
func (s *AuthService) Refresh(ctx context.Context, req model.RefreshRequest) (*model.AuthResponse, error)
func (s *AuthService) GetUser(ctx context.Context, userID string) (*model.User, error)
```

### JWT Claims

```go
// internal/model/claims.go
type Claims struct {
    jwt.RegisteredClaims
    Type string `json:"type"`  // "access" | "refresh"
}

// Constants
const (
    AccessExpiration  = 15 * time.Minute
    RefreshExpiration = 24 * time.Hour
    Issuer            = "api-gateway"
)
```

### Rate Limiter

```go
// internal/middleware/ratelimit.go
type Bucket struct {
    mu         sync.Mutex
    tokens     float64
    capacity   float64
    refillRate float64  // tokens per second
    lastRefill time.Time
}

type RateLimiter struct {
    mu      sync.Mutex
    buckets map[string]*Bucket
}

func NewRateLimiter() *RateLimiter
func (rl *RateLimiter) Allow(key string, capacity, refillRate float64) (bool, float64)
func (rl *RateLimiter) GetBucket(key string) *Bucket
func (rl *RateLimiter) Cleanup(maxAge time.Duration)  // goroutine: remove old buckets
```

### Middleware Signatures

```go
// internal/middleware/requestid.go
func RequestID(next http.Handler) http.Handler

// internal/middleware/logging.go
func Logging(logger *slog.Logger) func(http.Handler) http.Handler

// internal/middleware/metrics.go
func Metrics() func(http.Handler) http.Handler
// Registers: http_requests_total (counter), http_request_duration_seconds (histogram)

// internal/middleware/auth.go
func Auth(jwtSecret []byte) func(http.Handler) http.Handler
// Extracts userID from JWT, sets in context

// internal/middleware/ratelimit.go
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler
// Keys: "ip:<ip>", "user:<userID>", "endpoint:login:<ip>"
```

### HTTP Handlers

```go
// internal/handler/auth.go
func RegisterHandler(authService *service.AuthService) http.HandlerFunc
func LoginHandler(authService *service.AuthService) http.HandlerFunc
func RefreshHandler(authService *service.AuthService) http.HandlerFunc

// internal/handler/users.go
func GetCurrentUserHandler(authService *service.AuthService) http.HandlerFunc

// internal/handler/health.go
func HealthzHandler(store store.Store) http.HandlerFunc

// internal/handler/limits.go
func LimitsStatusHandler(limiter *middleware.RateLimiter) http.HandlerFunc

// internal/handler/spa.go
func SPAHandler(fs embed.FS) http.HandlerFunc
```

### Server Composition

```go
// internal/server/server.go
type Server struct {
    httpServer *http.Server
    router     *chi.Mux
    store      store.Store
    authService *service.AuthService
    limiter    *middleware.RateLimiter
    logger     *slog.Logger
}

func NewServer(cfg Config) *Server

type Config struct {
    Port       string
    JWTSecret  []byte
    AdminEmail string
    AdminPass  string
}

func (s *Server) Start(ctx context.Context) error
func (s *Server) Shutdown(ctx context.Context) error
```

### Request/Response Models

```go
// internal/model/auth.go
type RegisterRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type LoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type AuthResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
}

type RefreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}

type UserResponse struct {
    ID    string `json:"id"`
    Email string `json:"email"`
}
```

## Middleware Chain Order

```go
r := chi.NewRouter()

// Global middleware (all routes)
r.Use(middleware.RequestID)
r.Use(middleware.Logging(logger))
r.Use(middleware.Metrics())

// Public routes (no auth)
r.Get("/healthz", handler.HealthzHandler(store))
r.Get("/metrics", promhttp.Handler().ServeHTTP)

// Auth routes (no auth middleware, but rate limited)
r.Route("/api/v1/auth", func(r chi.Router) {
    r.Use(middleware.RateLimit(limiter))  // IP + login endpoint limits
    r.Post("/register", handler.RegisterHandler(authService))
    r.Post("/login", handler.LoginHandler(authService))
    r.Post("/refresh", handler.RefreshHandler(authService))
})

// Protected routes (auth + rate limit)
r.Group(func(r chi.Router) {
    r.Use(middleware.Auth(jwtSecret))
    r.Use(middleware.RateLimit(limiter))  // IP + user limits
    
    r.Get("/api/v1/users/me", handler.GetCurrentUserHandler(authService))
    r.Get("/api/v1/limits/status", handler.LimitsStatusHandler(limiter))
})

// SPA fallback (catch-all for non-API routes)
r.Get("/*", handler.SPAHandler(staticFiles))
```

## Server Timeouts

```go
httpServer := &http.Server{
    Addr:              ":8080",
    Handler:           router,
    ReadTimeout:       15 * time.Second,
    ReadHeaderTimeout: 5 * time.Second,
    WriteTimeout:      15 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```

## Graceful Shutdown

```go
// cmd/gateway/main.go
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

go func() {
    if err := srv.Start(ctx); err != nil && err != http.ErrServerClosed {
        log.Fatal(err)
    }
}()

<-ctx.Done()
log.Println("shutdown signal received")

shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := srv.Shutdown(shutdownCtx); err != nil {
    log.Fatal(err)
}
log.Println("server stopped gracefully")
```

## Rate Limiting Details

### Token Bucket Algorithm

```go
func (b *Bucket) Allow() bool {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    now := time.Now()
    elapsed := now.Sub(b.lastRefill).Seconds()
    b.tokens = min(b.capacity, b.tokens + elapsed * b.refillRate)
    b.lastRefill = now
    
    if b.tokens < 1 {
        return false
    }
    b.tokens--
    return true
}
```

### Rate Limit Keys

- **IP bucket**: `ip:<client-ip>` → capacity: 100, refill: 100/60 per sec
- **User bucket**: `user:<userID>` → capacity: 60, refill: 60/60 per sec
- **Login endpoint**: `endpoint:login:<client-ip>` → capacity: 10, refill: 10/60 per sec

### Rate Limit Headers

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1640995200  (Unix timestamp)
```

### 429 Response

```
HTTP/1.1 429 Too Many Requests
Retry-After: 60
Content-Type: application/json

{"error": "rate limit exceeded"}
```

### Cleanup Goroutine

```go
func (rl *RateLimiter) StartCleanup(ctx context.Context, maxAge time.Duration) {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            rl.Cleanup(maxAge)
        }
    }
}

func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    now := time.Now()
    for key, bucket := range rl.buckets {
        bucket.mu.Lock()
        if now.Sub(bucket.lastRefill) > maxAge {
            delete(rl.buckets, key)
        }
        bucket.mu.Unlock()
    }
}
```

## Auth Service Details

### Registration Flow

```go
func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
    // 1. Validate email format
    if !isValidEmail(req.Email) {
        return nil, ErrInvalidEmail
    }
    
    // 2. Validate password strength (min 8 chars)
    if len(req.Password) < 8 {
        return nil, ErrWeakPassword
    }
    
    // 3. Check if email exists
    if _, err := s.store.GetByEmail(ctx, req.Email); err == nil {
        return nil, ErrEmailExists
    }
    
    // 4. Hash password (bcrypt cost 12)
    hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.bcryptCost)
    if err != nil {
        return nil, err
    }
    
    // 5. Create user
    user := &model.User{
        ID:           uuid.NewString(),
        Email:        req.Email,
        PasswordHash: string(hash),
        CreatedAt:    time.Now(),
    }
    
    if err := s.store.Create(ctx, user); err != nil {
        return nil, err
    }
    
    return user, nil
}
```

### Login Flow

```go
func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error) {
    // 1. Get user by email
    user, err := s.store.GetByEmail(ctx, req.Email)
    if err != nil {
        return nil, ErrInvalidCredentials
    }
    
    // 2. Compare password
    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
        return nil, ErrInvalidCredentials
    }
    
    // 3. Generate tokens
    accessToken, err := s.generateToken(user.ID, "access", model.AccessExpiration)
    if err != nil {
        return nil, err
    }
    
    refreshToken, err := s.generateToken(user.ID, "refresh", model.RefreshExpiration)
    if err != nil {
        return nil, err
    }
    
    return &model.AuthResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
    }, nil
}

func (s *AuthService) generateToken(userID, tokenType string, duration time.Duration) (string, error) {
    claims := &model.Claims{
        RegisteredClaims: jwt.RegisteredClaims{
            Issuer:    model.Issuer,
            Subject:   userID,
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
        Type: tokenType,
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.jwtSecret)
}
```

### Refresh Flow

```go
func (s *AuthService) Refresh(ctx context.Context, req model.RefreshRequest) (*model.AuthResponse, error) {
    // 1. Parse token
    claims := &model.Claims{}
    token, err := jwt.ParseWithClaims(req.RefreshToken, claims, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, ErrInvalidToken
        }
        return s.jwtSecret, nil
    })
    
    if err != nil || !token.Valid {
        return nil, ErrInvalidToken
    }
    
    // 2. Validate token type
    if claims.Type != "refresh" {
        return nil, ErrInvalidToken
    }
    
    // 3. Validate issuer
    if claims.Issuer != model.Issuer {
        return nil, ErrInvalidToken
    }
    
    // 4. Generate new tokens
    accessToken, err := s.generateToken(claims.Subject, "access", model.AccessExpiration)
    if err != nil {
        return nil, err
    }
    
    refreshToken, err := s.generateToken(claims.Subject, "refresh", model.RefreshExpiration)
    if err != nil {
        return nil, err
    }
    
    return &model.AuthResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
    }, nil
}
```

## Prometheus Metrics

```go
// internal/middleware/metrics.go
var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "path", "status"},
    )
    
    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
        },
        []string{"method", "path"},
    )
)

func init() {
    prometheus.MustRegister(httpRequestsTotal)
    prometheus.MustRegister(httpRequestDuration)
}

func Metrics() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Wrap response writer to capture status
            wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
            next.ServeHTTP(wrapped, r)
            
            duration := time.Since(start).Seconds()
            status := strconv.Itoa(wrapped.status)
            
            httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
            httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
        })
    }
}
```

## Structured Logging

```go
// internal/middleware/logging.go
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Extract request ID from context
            requestID := r.Context().Value(requestIDKey).(string)
            
            // Wrap response writer to capture status
            wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
            next.ServeHTTP(wrapped, r)
            
            duration := time.Since(start)
            
            logger.InfoContext(r.Context(), "request",
                "method", r.Method,
                "path", r.URL.Path,
                "status", wrapped.status,
                "duration_ms", duration.Milliseconds(),
                "request_id", requestID,
            )
        })
    }
}
```

## Request ID

```go
// internal/middleware/requestid.go
type contextKey string

const requestIDKey contextKey = "requestID"

func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Use existing X-Request-ID or generate new one
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            id = uuid.NewString()
        }
        
        // Set in context
        ctx := context.WithValue(r.Context(), requestIDKey, id)
        
        // Set in response header
        w.Header().Set("X-Request-ID", id)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Frontend Integration

### embed.go

```go
// embed.go
package main

import "embed"

//go:embed web/dist/*
var staticFiles embed.FS
```

### SPA Fallback Handler

```go
// internal/handler/spa.go
func SPAHandler(fs embed.FS) http.HandlerFunc {
    fileServer := http.FileServer(http.FS(fs))
    
    return func(w http.ResponseWriter, r *http.Request) {
        path := r.URL.Path
        
        // Try to serve static file
        if _, err := fs.Open("web/dist" + path); err == nil {
            fileServer.ServeHTTP(w, r)
            return
        }
        
        // Fallback to index.html for SPA routing
        r.URL.Path = "/"
        fileServer.ServeHTTP(w, r)
    }
}
```

### Router Integration

```go
// Non-API routes → SPA fallback
r.Get("/*", handler.SPAHandler(staticFiles))
```

## Docker Multi-Stage Build

### Dockerfile

```dockerfile
# Stage 1: Build frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway

# Stage 3: Runtime
FROM alpine:3.20
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=go-builder /app/gateway .
EXPOSE 8080
CMD ["./gateway"]
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  gateway:
    build: .
    ports:
      - "8080:8080"
    environment:
      - JWT_SECRET=${JWT_SECRET:-super-secret-key-change-me}
      - ADMIN_EMAIL=admin@example.com
      - ADMIN_PASSWORD=admin1234
    restart: unless-stopped
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Store operations (Create, GetByEmail, GetByID) | Table-driven tests with MemoryStore |
| Unit | Auth service (Register, Login, Refresh) | Mock store, verify JWT claims |
| Unit | Rate limiter (Allow, Cleanup) | Concurrent access tests, bucket refill |
| Unit | Middleware (auth, ratelimit) | httptest.NewRecorder, verify headers/status |
| Integration | Full request flow (register → login → protected endpoint) | httptest.Server, real HTTP calls |
| Integration | Rate limit enforcement (100 requests → 429) | Loop requests, verify 429 |
| Integration | SPA fallback (static files, index.html) | httptest.Server, verify responses |
| E2E | Docker compose up → curl flow | Manual or scripted curl commands |

### Test Coverage Target

- **Minimum**: 80% coverage
- **Critical paths**: Auth middleware, rate limiter, store operations
- **Run**: `go test ./... -coverprofile=coverage.out`

## Threat Matrix

| Boundary | Minimum adversarial cases | Applicability | Design response | Planned RED tests |
|---|---|---|---|---|
| Documentation-like paths | N/A — no file upload or path traversal | N/A: no file operations | N/A | N/A |
| Git repository selection | N/A — no git operations | N/A: no VCS integration | N/A | N/A |
| Commit state | N/A — no git operations | N/A: no VCS integration | N/A | N/A |
| Push state | N/A — no git operations | N/A: no VCS integration | N/A | N/A |
| PR commands | N/A — no PR automation | N/A: no PR integration | N/A | N/A |

**Note**: This project is an HTTP API gateway with no shell commands, subprocesses, VCS operations, or file system writes beyond static asset serving. All routing is HTTP-based with Chi router. Threat matrix is not applicable.

## Migration / Rollout

No migration required — greenfield project.

### Deployment Steps

1. Set `JWT_SECRET` environment variable (required)
2. Run `docker compose up -d`
3. Verify health: `curl http://localhost:8080/healthz`
4. Register admin: already seeded on startup
5. Test login: `curl -X POST http://localhost:8080/api/v1/auth/login -d '{"email":"admin@example.com","password":"admin1234"}'`

### Rollback

Single container: `docker compose down && docker compose up -d` reverts to previous image.

## Implementation Order

Suggested order for task breakdown (dependencies flow top-down):

1. **Store layer** (model/user.go, store/store.go, store/memory.go, store/seed.go)
   - Foundation: all other layers depend on Store interface
   
2. **Models** (model/claims.go, model/auth.go, model/bucket.go)
   - Define data structures before implementing logic
   
3. **Auth service** (service/auth.go)
   - Business logic: Register, Login, Refresh, GetUser
   - Depends on Store + Models
   
4. **Middleware** (middleware/requestid.go, middleware/logging.go, middleware/metrics.go, middleware/auth.go, middleware/ratelimit.go)
   - Cross-cutting concerns: depends on Models
   
5. **Handlers** (handler/auth.go, handler/users.go, handler/health.go, handler/limits.go, handler/spa.go)
   - HTTP layer: depends on Service + Middleware
   
6. **Server assembly** (server/server.go, cmd/gateway/main.go)
   - Wiring: compose router, middleware, handlers, graceful shutdown
   
7. **Frontend** (web/src/pages/*, web/src/components/*, embed.go)
   - React SPA: depends on API endpoints being stable
   
8. **Docker** (Dockerfile, docker-compose.yml)
   - Packaging: depends on Go + Frontend builds
   
9. **README** (README.md)
   - Documentation: architecture diagram, curl demo script

## Open Questions

- [ ] **Admin password strength**: Should we enforce stronger passwords for admin seed (e.g., 12+ chars, special chars) or keep demo-friendly (8+ chars)?
- [ ] **Rate limit status endpoint auth**: Should `/api/v1/limits/status` require authentication (shows user's own buckets) or be public (shows IP bucket only)?
- [ ] **JWT refresh token rotation**: Should we implement refresh token rotation (invalidate old refresh token on use) or keep simple (allow reuse until expiry)?
- [ ] **Frontend token storage**: localStorage is vulnerable to XSS. Should we use httpOnly cookies instead (more secure, but more complex)?

## Dependencies

### Go Modules

```
github.com/go-chi/chi/v5          # Router
github.com/golang-jwt/jwt/v5      # JWT
github.com/google/uuid            # UUID generation
github.com/prometheus/client_golang # Prometheus metrics
golang.org/x/crypto/bcrypt        # Password hashing
```

### Node.js (Frontend)

```
react, react-dom, react-router-dom
vite, @vitejs/plugin-react
typescript
```

## Success Criteria

- [ ] `docker compose up` starts gateway with single command
- [ ] `curl` flow: register → login → token → `/api/v1/users/me` (200) → exceed limit (429)
- [ ] `/metrics` returns real Prometheus data after requests
- [ ] `/api/v1/limits/status` returns JSON with per-bucket usage
- [ ] `go test ./...` passes, coverage ≥ 80%
- [ ] Frontend displays real JWT claims + rate limit bars (not mock data)
- [ ] `GET /healthz` returns 200 with store readiness status
- [ ] README includes architecture diagram + curl demo script
