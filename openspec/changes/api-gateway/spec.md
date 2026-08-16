# API Gateway — Delta Specs

## jwt-auth

| ID | Requirement | Criteria |
|---|---|---|
| JWT-AUTH-001 | User Registration | POST /api/v1/auth/register, bcrypt password, returns 201 |
| JWT-AUTH-002 | User Login | POST /api/v1/auth/login, returns {access_token, refresh_token} |
| JWT-AUTH-003 | Token Refresh | POST /api/v1/auth/refresh, validates refresh token, returns new pair |
| JWT-AUTH-004 | JWT Middleware | Validates signature/exp/iss; extracts sub and type; 401 on failure |
| JWT-AUTH-005 | Protected Endpoints | GET /api/v1/users/me requires Bearer access token |
| JWT-AUTH-006 | JWT Claims | iss, sub, exp, iat, type (access\|refresh) |
| JWT-AUTH-007 | JWT Secret | Fail-fast if JWT_SECRET missing from env |
| JWT-AUTH-008 | Token Expiration | Access: 15min, Refresh: 24h |

**Scenarios**: successful registration (201), duplicate email (409), successful login (200), invalid credentials (401), valid refresh (200), expired refresh (401), access protected endpoint (200), invalid token (401), missing JWT_SECRET (fail-fast).

## rate-limiting

| ID | Requirement | Criteria |
|---|---|---|
| RATE-001 | IP Rate Limit | 100 req/min per IP, token bucket |
| RATE-002 | User Rate Limit | 60 req/min per authenticated user |
| RATE-003 | Login Rate Limit | 10 req/min per IP on /auth/login |
| RATE-004 | Rate Limit Headers | X-RateLimit-Limit/Remaining/Reset on all responses |
| RATE-005 | Status Endpoint | GET /api/v1/limits/status returns per-bucket JSON |
| RATE-006 | 429 Response | 429 + Retry-After when limit exceeded |
| RATE-007 | Token Bucket | sync.Mutex + map[string]*Bucket |
| RATE-008 | Bucket Isolation | IP, user, endpoint tracked independently |

**Scenarios**: IP within bounds (allowed, remaining header), IP exceeded (429), user exceeded (429), login brute-force (429), status endpoint (200 JSON), headers present, bucket reset after window.

## structured-logging

| ID | Requirement | Criteria |
|---|---|---|
| LOG-001 | JSON Handler | slog JSON handler for all output |
| LOG-002 | Request ID Generation | UUID v4 per request |
| LOG-003 | Request ID Propagation | X-Request-ID in/out |
| LOG-004 | Request Logging | method, path, status, duration, request_id |
| LOG-005 | Log Levels | DEBUG, INFO, WARN, ERROR |
| LOG-006 | Context Propagation | Request ID in context |

**Scenarios**: ID generated (new UUID), ID propagated (echoed back), request logged (all fields), JSON format, error logged (ERROR level).

## prometheus-metrics

| ID | Requirement | Criteria |
|---|---|---|
| METRICS-001 | Request Counter | Counter: method, path, status |
| METRICS-002 | Latency Histogram | Histogram: method, path; buckets 10ms-5s |
| METRICS-003 | Metrics Endpoint | GET /metrics, Prometheus format |
| METRICS-004 | No Auth Required | /metrics unauthenticated |
| METRICS-005 | Middleware Integration | Auto-record every request |

**Scenarios**: endpoint accessible (200), counter incremented, histogram recorded, multiple status codes tracked.

## health-check

| ID | Requirement | Criteria |
|---|---|---|
| HEALTH-001 | Liveness Probe | GET /healthz → 200 {status: "ok"} |
| HEALTH-002 | Readiness Check | Checks store availability |
| HEALTH-003 | Readiness Response | 200 if ready, 503 if not |
| HEALTH-004 | No Auth Required | /healthz unauthenticated |

**Scenarios**: liveness success (200), readiness success (200), readiness failure (503), no auth needed.

## user-store

| ID | Requirement | Criteria |
|---|---|---|
| STORE-001 | Store Interface | Create, GetByEmail, GetByID |
| STORE-002 | MemoryStore | sync.RWMutex in-memory impl |
| STORE-003 | User Model | ID (UUID), Email, PasswordHash, CreatedAt |
| STORE-004 | Admin Seed | Seed admin user on startup |
| STORE-005 | Email Uniqueness | Enforce unique emails |
| STORE-006 | Thread Safety | Concurrent-safe operations |

**Scenarios**: create user, duplicate email (error), get by email, get non-existent (error), get by ID, admin seeded, concurrent access.

## embedded-frontend

| ID | Requirement | Criteria |
|---|---|---|
| FE-001 | Build Output | Vite → web/dist/ |
| FE-002 | Embed FS | //go:embed web/dist/ |
| FE-003 | SPA Fallback | Non-API, non-metrics → index.html |
| FE-004 | Static Assets | Correct MIME types |
| FE-005 | Login Page | Form → /api/v1/auth/login |
| FE-006 | Register Page | Form → /api/v1/auth/register |
| FE-007 | API Explorer | Display JWT claims |
| FE-008 | Rate Limit Bars | Visual bars from /api/v1/limits/status |
| FE-009 | Health Status | Display /healthz status |
| FE-010 | Same Origin | No CORS needed |
| FE-011 | No Demo Data | Real API data only |

**Scenarios**: serve root (index.html), static asset (correct MIME), SPA fallback, API route not caught, login form, API explorer shows claims, rate limit bars, health status, no demo data.
