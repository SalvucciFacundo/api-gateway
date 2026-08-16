# Proposal: API Gateway — Auth + Rate Limiting + Observability

## Intent

Production-ready HTTP gateway demo showcasing middleware-centric architecture: JWT auth, rate limiting, structured logging, Prometheus metrics, embedded React SPA. Single binary, single container, zero CORS friction. Proves understanding of the full HTTP stack that production traffic flows through.

## Scope

### In Scope
- JWT auth (register/login/refresh + middleware validation)
- Rate limiting (token bucket, in-memory, per-IP + per-user + per-endpoint)
- Structured logging (JSON, request ID propagation)
- Prometheus metrics (counters by endpoint/status, latency histograms)
- Health check (liveness + readiness)
- Users store (in-memory with `Store` interface for Postgres swap)
- Embedded React SPA via `embed.FS` (same-origin, no CORS)
- Docker multi-stage build + `docker-compose.yml`
- README with architecture diagram + curl demo script
- Tests via `httptest`, coverage ≥ 80%

### Out of Scope
- CORS (same-origin eliminates need)
- Redis-backed rate limiting (next iteration)
- Refresh token rotation/revocation (simple refresh in base)
- Plan-based rate limits (free/premium)
- Postgres store implementation
- CI/CD pipeline (deferred)

## Capabilities

### New Capabilities
- `jwt-auth`: registration, login, refresh token issuance, JWT middleware validation
- `rate-limiting`: token bucket algorithm, per-IP/per-user/per-endpoint buckets, headers + status endpoint
- `structured-logging`: JSON slog handler, request ID generation, header propagation
- `prometheus-metrics`: HTTP request counters, latency histograms, /metrics scrape endpoint
- `health-check`: liveness probe, readiness probe with store check
- `user-store`: in-memory user persistence with swappable interface
- `embedded-frontend`: React SPA served via embed.FS, SPA fallback routing

### Modified Capabilities
None (greenfield project).

## Approach

| Decision | Choice | Rationale |
|---|---|---|
| Router | Chi (implements `http.Handler`) | Middleware composition, stdlib-compatible |
| Server | `http.Server` with explicit timeouts + graceful shutdown | Production hardening |
| JWT lib | `golang-jwt/jwt/v5` | Access (15min) + Refresh (24h), claims: `iss/sub/exp/iat/type` |
| Rate limit | Token bucket, `sync.Mutex` + `map[string]*Bucket` | Simple, testable, documented upgrade path to Redis |
| Rate limits | IP: 100/min, User: 60/min, `/auth/login`: 10/min | Anti-brute-force on login, reasonable demo defaults |
| Rate limit API | Headers `X-RateLimit-{Limit,Remaining,Reset}` + `GET /api/v1/limits/status` | Frontend reads JSON (can't parse Prometheus) |
| Logging | `slog` JSON handler, UUID v4 request ID via `X-Request-ID` | Structured, leveled, propagates to response |
| Store | `Store` interface, `MemoryStore` impl with `sync.RWMutex` | Demo-first, Postgres-ready |
| Frontend | Vite build → `embed.FS`, SPA catch-all for non-`/api/*` routes | Same-origin, single binary |
| Docker | Multi-stage: `node:22-alpine` (build) → `golang:1.24-alpine` (binary) → `alpine:3.20` (runtime) | Small image, reproducible |

### Middleware Chain Order
`requestID → logging → metrics → auth → rateLimit → handler`

### Project Layout
```
cmd/gateway/main.go              # entrypoint, wiring, graceful shutdown
internal/
  handler/                        # HTTP handlers (auth, users, health, limits, spa)
  middleware/                     # auth, ratelimit, logging, metrics, requestid
  service/                        # business logic (auth service)
  store/                          # Store interface + MemoryStore
  model/                          # domain types (User, Claims, Bucket)
web/                              # React SPA source (Vite)
  dist/                           # build output (embedded)
docker-compose.yml
Dockerfile
```

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Mutex contention under load | Low (demo scope) | Documented upgrade path to Redis; `pprof` hooks |
| In-memory state loss on restart | Certain | Acceptable for demo; `Store` interface enables Postgres swap |
| `embed.FS` stale assets after frontend edit | Low | Docker multi-stage rebuilds always; local dev with `go generate` |
| JWT secret misconfiguration | Medium | Fail-fast on startup if `JWT_SECRET` env missing |

## Rollback Plan

Single binary, single container. `docker compose down && docker compose up -d` reverts to previous image. Code-level: `git revert` on `main`, Dokploy redeploys automatically from branch.

## Dependencies

- Go 1.24+
- Node.js 22+ (frontend build)
- Docker + docker-compose
- `JWT_SECRET` environment variable (required at runtime)

## Success Criteria

- [ ] `docker compose up` starts gateway with single command
- [ ] `curl` flow: register → login → token → `/api/v1/users/me` (200) → exceed limit (429)
- [ ] `/metrics` returns real Prometheus data after requests
- [ ] `/api/v1/limits/status` returns JSON with per-bucket usage
- [ ] `go test ./...` passes, coverage ≥ 80%
- [ ] Frontend displays real JWT claims + rate limit bars (not mock data)
- [ ] `GET /healthz` returns 200 with store readiness status
- [ ] README includes architecture diagram + curl demo script
