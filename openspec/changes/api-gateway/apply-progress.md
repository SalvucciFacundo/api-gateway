# Apply Progress: API Gateway — PR1 + PR2 (cumulative)

**Change**: api-gateway
**Mode**: Strict TDD
**Chain strategy**: stacked-to-main
**Artifact store**: hybrid (OpenSpec + Engram)
**Review budget**: 800 changed lines (session)

## Status

PR1 (T1–T7) and PR2 (T8–T9) complete. `go test ./... -race -cover` passes; coverage
per package: middleware 100%, model 100%, service 84.6%, store 89.2%.
`go build ./...`, `go vet ./...` exit 0, `gofmt -l .` clean.

## Completed Tasks (cumulative)

### PR1 — Foundation: models + store (T1–T7)

- [x] T1: `go.mod`/`go.sum` — chi/v5, golang-jwt/v5, google/uuid, prometheus/client_golang, x/crypto/bcrypt
- [x] T2: `internal/model/user.go` + test (STORE-003)
- [x] T3: `internal/model/claims.go` + test (JWT-AUTH-006,008)
- [x] T4: `internal/model/auth.go` + test (JSON round-trip)
- [x] T5: `internal/model/bucket.go` + test (RATE-007)
- [x] T6: `internal/store/store.go` + `memory.go` + test (STORE-001/002/005/006 + `-race`)
- [x] T7: `internal/store/seed.go` + test (STORE-004 idempotent)

### PR2 — Auth service + auth middleware (T8–T9)

- [x] T8: `internal/service/auth.go` + test — `AuthService{store,jwtSecret,bcryptCost:12}`; Register/Login/Refresh/GetUser/generateToken. ✅ JWT-AUTH-001..003,005,008.
- [x] T9: `internal/middleware/auth.go` + test — `Auth(jwtSecret)`: Bearer→validate sig/exp/iss→sub in ctx; 401. ✅ JWT-AUTH-004/005.

## Files Changed

**PR1**: `go.mod`, `go.sum`, `internal/model/{user,claims,auth,bucket}.go` + tests, `internal/store/{store,memory,seed}.go` + tests.

**PR2**: `internal/service/{auth.go,auth_test.go}`, `internal/middleware/{auth.go,auth_test.go}`, `openspec/changes/api-gateway/tasks.md` (T8/T9 `[x]`).

## TDD Cycle Evidence (PR1)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| T1 | — (scaffold) | — | N/A (new) | ➖ N/A (config) | ✅ `go build` ok | ➖ Single | ✅ Clean |
| T2 | `internal/model/user_test.go` | Unit | N/A (new) | ✅ `undefined: User` | ✅ Passed | ➖ Single (structural) | ➖ None needed |
| T3 | `internal/model/claims_test.go` | Unit | N/A (new) | ✅ undefined consts/type | ✅ Passed | ✅ 2 cases | ✅ Clean |
| T4 | `internal/model/auth_test.go` | Unit | N/A (new) | ✅ undefined types | ✅ Passed | ✅ 5 structs | ✅ Clean |
| T5 | `internal/model/bucket_test.go` | Unit | N/A (new) | ✅ `undefined: NewBucket` | ✅ Passed | ✅ 3 cases | ✅ Clean |
| T6 | `internal/store/memory_test.go` | Unit | N/A (new) | ✅ undefined store symbols | ✅ Passed | ✅ 8 cases | ✅ Clean |
| T7 | `internal/store/seed_test.go` | Unit | N/A (new) | ✅ `undefined: SeedAdmin` | ✅ Passed | ✅ 2 cases | ✅ Clean |

## TDD Cycle Evidence (PR2)

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| T8 | `internal/service/auth_test.go` | Unit | N/A (new pkg; full suite green baseline) | ✅ Written (compile fail: undefined `AuthService`/`NewAuthService`/`Err*`) | ✅ Passed (`go test -race`) | ✅ 22 cases (validation, dup email, invalid creds, refresh invalid×5, generateToken claims) | ✅ Clean |
| T9 | `internal/middleware/auth_test.go` | Unit | N/A (new pkg) | ✅ Written (compile fail: undefined `authenticate`/`Auth`/`UserIDFromContext`) | ✅ Passed (`go test -race`) | ✅ 15 cases (11 authenticate + 4 middleware, incl. alg-confusion/none) | ✅ Clean |

## Work Unit Evidence (PR1 slice)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/model/... ./internal/store/... -race` → `ok` (both packages, 0 failures, race detector clean) |
| Runtime harness command/scenario and exact result | `go build ./...` → exit 0. `go vet ./...` → exit 0. `gofmt -l .` → no output. Runtime server boundary is N/A: no server/main exists until PR5. |
| Rollback boundary | Revert the PR1 commits `19eaed2..4c7c52f`, or delete `internal/model/*`, `internal/store/*` and restore `go.mod`/`go.sum`. |

## Work Unit Evidence (PR2 slice)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/service/... ./internal/middleware/... -race` → `ok` both packages, exit 0 (service 84.6%, middleware 100% coverage) |
| Runtime harness command/scenario and exact result | httptest round-trip through `Auth` middleware: `go test ./internal/middleware/... -run TestAuthMiddleware -race -v` → valid token → 200 with `user-123` in context; missing/invalid/refresh token → 401 with `{"error":"unauthorized"}` + `WWW-Authenticate: Bearer`. `go build ./...` → exit 0. Service (T8) has no independent runtime boundary (pure business logic, unit-tested); its HTTP path is exercised via the middleware httptest flow. |
| Rollback boundary | Revert `internal/service/` and `internal/middleware/` directories + uncheck T8/T9 in `openspec/changes/api-gateway/tasks.md`. No other files touched. |

## Test Summary (PR2)

- **Total test functions written**: 12 (10 service + 2 middleware) + 27 named subtests
- **Total passing**: all (`go test ./... -race` → `ok`)
- **Layers used**: Unit (all); httptest HTTP-level for middleware
- **Approval tests** (refactoring): None — no refactoring of existing code (all new packages)
- **Pure functions created**: `authenticate()` (middleware), `generateToken`/`generateTokenPair`/`parseToken` (service)

## Deviations from Design

**PR1**

1. **Bucket location**: placed in `internal/model/bucket.go` per task T5; the design's "Rate Limiter" section shows `type Bucket struct` under `internal/middleware/ratelimit.go` — a copy/paste inconsistency. `middleware.RateLimiter` (PR3) will import `model.Bucket`.
2. **Sentinel errors**: added `store.ErrEmailExists`, `store.ErrUserNotFound`, `store.ErrInvalidUser` (not enumerated in design) to implement STORE-005 semantics.
3. **SeedAdmin return**: returns the existing user on an idempotent re-call (design left return semantics unspecified).
4. **bcrypt cost in tests**: store tests use `bcrypt.MinCost` for speed; cost 12 is the AuthService default (PR2).

**PR2**

1. `NewAuthService` parameter named `s` (not `store`) to avoid shadowing the `store` package import.
2. Service tests use the real in-memory `MemoryStore` instead of a hand-rolled mock (stdlib-only convention, no gomock; `MemoryStore` is already race-tested and fast).
3. Added `authenticate()` as an unexported pure helper in the middleware so token validation is unit-testable without HTTP plumbing; `Auth` wraps it.
4. Added a `WWW-Authenticate: Bearer` header on 401 (RFC 6750 correctness; not specified in design).
5. Middleware rejects tokens with empty `subject` (defensive; "extracts sub" is the point of the middleware).
6. Token type strings `"access"`/`"refresh"` remain literals per design (minor duplication across service/middleware; noted for future constant extraction).

## Issues Found

1. **`contextKey` type name collision risk (PR3)**: `internal/middleware/auth.go` defines `type contextKey string`. The design's `requestid.go` (PR3/T10) also declares `type contextKey string`; the PR3 implementer must reuse the existing type and only add the `requestIDKey` const, or the package will fail to compile.
2. **PR2 slice slightly over review budget**: authored 821 lines (code + tests) + 2 in `tasks.md` = 823 changed lines, ~2.9% over the 800-line session budget. Flagged for reviewer awareness; no functional impact.

## Verification

```
$ go test -count=1 -race ./...
ok  github.com/SalvucciFacundo/api-gateway/internal/middleware  1.015s
ok  github.com/SalvucciFacundo/api-gateway/internal/model       1.011s
ok  github.com/SalvucciFacundo/api-gateway/internal/service     1.099s
ok  github.com/SalvucciFacundo/api-gateway/internal/store       1.041s

$ go test -count=1 -cover ./...
middleware  coverage: 100.0% of statements
model       coverage: 100.0% of statements
service     coverage: 84.6% of statements
store       coverage: 89.2% of statements

$ go build ./...   → exit 0
$ go vet ./...     → exit 0
$ gofmt -l internal/service internal/middleware → no output
```

## Remaining Tasks (PR3+)

- [ ] T10–T13: middleware observability + rate limiting
- [ ] T14–T19: handlers + server wiring + main
- [ ] T20–T22: frontend + Docker + README
