# Delta for Rate Limiting

## ADDED Requirements

| ID | Requirement | Criteria |
|---|---|---|
| RATE-001 | IP Rate Limit | 100 requests per minute per IP address using token bucket algorithm |
| RATE-002 | User Rate Limit | 60 requests per minute per authenticated user |
| RATE-003 | Login Rate Limit | 10 requests per minute per IP for /api/v1/auth/login endpoint (anti-brute-force) |
| RATE-004 | Rate Limit Headers | Responses MUST include X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset headers |
| RATE-005 | Status Endpoint | GET /api/v1/limits/status returns JSON with per-bucket usage for authenticated user |
| RATE-006 | 429 Response | When limit exceeded, return 429 Too Many Requests with Retry-After header |
| RATE-007 | Token Bucket | In-memory implementation using sync.Mutex and map[string]*Bucket |
| RATE-008 | Bucket Isolation | Each bucket (IP, user, endpoint) tracked independently |

## Scenarios

### Scenario: IP Rate Limit Within Bounds
- GIVEN an IP address has made 50 requests in the last minute
- WHEN the 51st request is made
- THEN request is allowed
- AND X-RateLimit-Remaining header shows 49

### Scenario: IP Rate Limit Exceeded
- GIVEN an IP address has made 100 requests in the last minute
- WHEN the 101st request is made
- THEN response is 429 Too Many Requests
- AND Retry-After header indicates seconds until reset

### Scenario: User Rate Limit Exceeded
- GIVEN an authenticated user has made 60 requests in the last minute
- WHEN the 61st request is made
- THEN response is 429 Too Many Requests

### Scenario: Login Anti-Brute-Force
- GIVEN an IP has made 10 login attempts in the last minute
- WHEN the 11th login attempt is made
- THEN response is 429 Too Many Requests

### Scenario: Rate Limit Status Endpoint
- GIVEN an authenticated user with active rate limit buckets
- WHEN GET /api/v1/limits/status is called
- THEN response is 200 OK with JSON {ip: {limit, remaining, reset}, user: {limit, remaining, reset}}

### Scenario: Rate Limit Headers Present
- GIVEN any authenticated request
- WHEN the response is returned
- THEN headers include X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset

### Scenario: Bucket Reset After Window
- GIVEN an IP has exceeded its rate limit
- WHEN 60 seconds pass
- THEN the bucket is refilled and requests are allowed again
