# Apply Progress: API Gateway — PR1 (Foundation: models + store)

**Change**: api-gateway
**Mode**: Strict TDD
**PR Slice**: PR1 of 7 (stacked-to-main)
**Scope**: T1–T7
**Artifact store**: hybrid (OpenSpec + Engram)

## Status

T1–T7 complete. `go test ./... -race -cover` passes, coverage ≥ 80% on touched
packages (model 100%, store 89.2%). `go build ./...` compiles.

## Completed Tasks

- [x] T1: `go.mod`/`go.sum` — chi/v5, golang-jwt/v5, google/uuid, prometheus/client_golang, x/crypto/bcrypt
- [x] T2: `internal/model/user.go` + test (STORE-003)
- [x] T3: `internal/model/claims.go` + test (JWT-AUTH-006,008)
- [x] T4: `internal/model/auth.go` + test (JSON round-trip)
- [x] T5: `internal/model/bucket.go` + test (RATE-007)
- [x] T6: `internal/store/store.go` + `memory.go` + test (STORE-001/002/005/006 + `-race`)
- [x] T7: `internal/store/seed.go` + test (STORE-004)

## Files Changed

| File | Action | What Was Done |
|------|--------|---------------|
| `go.mod` | Created | Module `github.com/SalvucciFacundo/api-gateway` + 5 deps |
| `go.sum` | Created | Dependency checksums |
| `internal/model/user.go` | Created | `User{ID,Email,PasswordHash,CreatedAt}` |
| `internal/model/claims.go` | Created | `Claims{RegisteredClaims; Type}` + Access/Refresh/Issuer consts |
| `internal/model/auth.go` | Created | Register/Login/AuthResponse/Refresh/UserResponse |
| `internal/model/bucket.go` | Created | Token `Bucket` + `NewBucket` + `Allow()` |
| `internal/store/store.go` | Created | `Store` interface (Create/GetByEmail/GetByID) |
| `internal/store/memory.go` | Created | `MemoryStore` (RWMutex + email uniqueness) |
| `internal/store/seed.go` | Created | `SeedAdmin` (idempotent) |
| `internal/model/*_test.go` | Created | model package tests |
| `internal/store/*_test.go` | Created | store package tests |

## TDD Cycle Evidence

| Task | Test File | Layer | Safety Net | RED | GREEN | TRIANGULATE | REFACTOR |
|------|-----------|-------|------------|-----|-------|-------------|----------|
| T1 | — (scaffold) | — | N/A (new) | ➖ N/A (config) | ✅ `go build` ok | ➖ Single | ✅ Clean |
| T2 | `internal/model/user_test.go` | Unit | N/A (new) | ✅ `undefined: User` | ✅ Passed | ➖ Single (structural) | ➖ None needed |
| T3 | `internal/model/claims_test.go` | Unit | N/A (new) | ✅ undefined consts/type | ✅ Passed | ✅ 2 cases | ✅ Clean |
| T4 | `internal/model/auth_test.go` | Unit | N/A (new) | ✅ undefined types | ✅ Passed | ✅ 5 structs | ✅ Clean |
| T5 | `internal/model/bucket_test.go` | Unit | N/A (new) | ✅ `undefined: NewBucket` | ✅ Passed | ✅ 3 cases | ✅ Clean |
| T6 | `internal/store/memory_test.go` | Unit | N/A (new) | ✅ undefined store symbols | ✅ Passed | ✅ 8 cases | ✅ Clean |
| T7 | `internal/store/seed_test.go` | Unit | N/A (new) | ✅ `undefined: SeedAdmin` | ✅ Passed | ✅ 2 cases | ✅ Clean |

## Test Summary

- **Total tests written**: 12 (model 8, store 4 + subtests)
- **Total tests passing**: 12/12
- **Layers used**: Unit (12)
- **Approval tests** (refactoring): None — no refactoring tasks
- **Pure functions created**: `Bucket.Allow` (deterministic given injected `lastRefill`)

## Work Unit Evidence (PR1 slice)

| Evidence | Required value |
|---|---|
| Focused test command and exact result | `go test ./internal/model/... ./internal/store/... -race` → `ok` (both packages, 0 failures, race detector clean) |
| Runtime harness command/scenario and exact result | `go build ./...` → exit 0. `go vet ./...` → exit 0. `gofmt -l .` → no output. Runtime server boundary is N/A: no server/main exists until PR5. |
| Rollback boundary | `git revert` of commits `b17efe1..dd14458` (or delete `internal/model/*`, `internal/store/*`, restore `go.mod`/`go.sum`) — removes only T1–T7, no unrelated work. |

## Deviations from Design

1. **Bucket location**: placed in `internal/model/bucket.go` per task T5 and the design's package-structure block. The design's "Rate Limiter" section shows `type Bucket struct` under `// internal/middleware/ratelimit.go`; that is a copy/paste inconsistency. `middleware.RateLimiter` (PR3) will import `model.Bucket`.
2. **Sentinel errors**: added `store.ErrEmailExists`, `store.ErrUserNotFound`, `store.ErrInvalidUser` (not enumerated in design) — required to implement STORE-005 error semantics and avoid nil derefs.
3. **SeedAdmin return**: returns the existing user on an idempotent re-call (design left return semantics unspecified).
4. **bcrypt cost in tests**: store tests use `bcrypt.MinCost` (4) for speed; cost 12 is the AuthService default and lands in PR2.

## Issues Found

None.

## Verification

```
$ go test ./... -race -cover
ok  github.com/SalvucciFacundo/api-gateway/internal/model  1.009s  coverage: 100.0% of statements
ok  github.com/SalvucciFacundo/api-gateway/internal/store  1.035s  coverage: 89.2% of statements

$ go build ./...   → exit 0
$ go vet ./...     → exit 0
$ go mod verify    → all modules verified
```

## Remaining Tasks (PR2+)

- [ ] T8: `internal/service/auth.go` + test (JWT-AUTH-001..003,005,008)
- [ ] T9: `internal/middleware/auth.go` + test (JWT-AUTH-004/005)
- [ ] T10–T13: middleware observability + rate limiting
- [ ] T14–T19: handlers + server wiring + main
- [ ] T20–T22: frontend + Docker + README
