# Tasks: API Gateway — Auth + Rate Limiting + Observability

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | ~3,700 total (backend ~2,900, frontend ~670, ops/docs ~235) |
| Budget risk (session budget: 800) | High |
| Chained PRs recommended | Yes |
| Suggested split | PR1–PR7 (see work units) |
| Delivery strategy | auto-chain |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| PR | Unit | Focused test command | Runtime harness | Rollback boundary |
|----|------|---------------------|-----------------|-------------------|
| 1 | Foundation: models + store (T1–T7) | `go test ./internal/model/... ./internal/store/...` | `go build ./...` | revert T1–T7 |
| 2 | Auth service + auth middleware (T8–T9) | `go test ./internal/service/...` | httptest login/refresh flow | revert T8–T9 |
| 3 | Observability + rate limiting (T10–T13) | `go test ./internal/middleware/...` | httptest: 429 + headers | revert T10–T13 |
| 4 | HTTP handlers (T14–T17) | `go test ./internal/handler/...` | httptest: register→me | revert T14–T17 |
| 5 | Server wiring + main (T18–T19) | `go test ./internal/server/...` | `go run ./cmd/gateway` → curl /healthz | revert T18–T19 |
| 6 | Frontend SPA (T20) | `npm run build` | browser: login → claims + bars | revert T20 |
| 7 | Docker + README (T21–T22) | `docker compose build` | `docker compose up` → curl demo | revert T21–T22 |

STRICT TDD: write failing test (RED) before each production task; triangulate; run `go test ./...` after each task.

## Phase 1: Foundation — Models + Store (T1–T7)

- [x] T1: `go.mod`/`go.sum` — add chi/v5, golang-jwt/v5, google/uuid, prometheus/client_golang, x/crypto/bcrypt. ✅ `go build ./...` compiles.
- [x] T2: `internal/model/user.go` + test — `User{ID,Email,PasswordHash,CreatedAt}`. ✅ STORE-003 fields test.
- [x] T3: `internal/model/claims.go` + test — `Claims{RegisteredClaims; Type}`, consts Access 15m / Refresh 24h / Issuer. ✅ JWT-AUTH-006,008.
- [x] T4: `internal/model/auth.go` + test — Register/Login/AuthResponse/Refresh/UserResponse JSON round-trip. ✅ unmarshal test.
- [x] T5: `internal/model/bucket.go` + test — `Bucket{mu,tokens,capacity,refillRate,lastRefill}` + `Allow()`. ✅ RATE-007 refill/consume.
- [x] T6: `internal/store/store.go`+`memory.go` + test — Store iface; MemoryStore RWMutex + email uniqueness. ✅ STORE-001/002/005/006 + `-race`.
- [x] T7: `internal/store/seed.go` + test — `SeedAdmin(ctx,store,email,pass,cost)`. ✅ STORE-004 idempotent.

## Phase 2: Auth Core (T8–T9)

- [x] T8: `internal/service/auth.go` + test — `AuthService{store,jwtSecret,bcryptCost:12}`; Register/Login/Refresh/GetUser/generateToken. ✅ JWT-AUTH-001..003,005,008.
- [x] T9: `internal/middleware/auth.go` + test — `Auth(jwtSecret)`: Bearer→validate sig/exp/iss→sub in ctx; 401. ✅ JWT-AUTH-004/005.

## Phase 3: Middleware — Observability + Rate Limiting (T10–T13)

- [x] T10: `internal/middleware/requestid.go` + test — UUID v4 gen/propagate via X-Request-ID. ✅ LOG-002/003/006.
- [x] T11: `internal/middleware/logging.go` + test — slog JSON: method/path/status/duration_ms/request_id. ✅ LOG-004 (LOG-001 JSON handler deferred to server wiring T18).
- [x] T12: `internal/middleware/metrics.go` + test — counter + histogram + status-capturing responseWriter. ✅ METRICS-001/002/005.
- [x] T13: `internal/middleware/ratelimit.go` + test — RateLimiter Allow/GetBucket/Cleanup/StartCleanup; keys ip:/user:/endpoint:login:. ✅ RATE-001..004,006..008.

## Phase 4: HTTP Layer + Wiring (T14–T19)

- [x] T14: `internal/handler/auth.go`+`users.go` + test — Register/Login/Refresh/GetCurrentUser. ✅ 201/409/200/401 httptest.
- [x] T15: `internal/handler/health.go` + test — liveness 200 `{status:"ok"}`; readiness 200/503. ✅ HEALTH-001..004.
- [x] T16: `internal/handler/limits.go` + test — `{ip,user}:{limit,remaining,reset}` JSON. ✅ RATE-005.
- [x] T17: `internal/handler/spa.go` + test — static assets + index.html fallback. ✅ FE-002/003/004.
- [x] T18: `internal/server/server.go` + test — chain requestID→logging→metrics→auth→rateLimit→handler; timeouts; Start/Shutdown. ✅ integration: register→login→me, 429 loop, /metrics.
- [x] T19: `cmd/gateway/main.go`+`embed.go` — JWT_SECRET fail-fast; admin seed; graceful shutdown. ✅ JWT-AUTH-007.

## Phase 5: Frontend (T20)

- [x] T20: `web/` Vite SPA — LoginPage/RegisterPage/ExplorerPage, RateLimitBar, HealthStatus; tokens in localStorage; real API only (no mock). ✅ `npm run build` → `web/dist`; FE-001,005-011.

## Phase 6: Packaging + Docs (T21–T22)

- [x] T21: `Dockerfile`+`docker-compose.yml` — 3-stage build; JWT_SECRET/ADMIN_* env; port 8080. ⚠️ Implementation complete; Docker runtime validation pending because Docker is unavailable.
- [x] T22: `README.md` — architecture diagram, curl demo script, demo creds (admin@example.com/admin1234), localStorage-XSS note. ✅ Documentation complete; curl flow requires Docker/runtime validation.
