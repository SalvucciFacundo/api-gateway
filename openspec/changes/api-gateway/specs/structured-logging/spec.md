# Delta for Structured Logging

## ADDED Requirements

| ID | Requirement | Criteria |
|---|---|---|
| LOG-001 | JSON Handler | slog MUST use JSON handler for all log output |
| LOG-002 | Request ID Generation | Middleware generates UUID v4 for each incoming request |
| LOG-003 | Request ID Propagation | Request ID propagated via X-Request-ID header (input and output) |
| LOG-004 | Request Logging | Every request logged with: method, path, status code, duration, request_id |
| LOG-005 | Log Levels | Support DEBUG, INFO, WARN, ERROR levels via slog |
| LOG-006 | Context Propagation | Request ID stored in context and accessible to handlers |

## Scenarios

### Scenario: Request ID Generated
- GIVEN an incoming request without X-Request-ID header
- WHEN the request passes through requestID middleware
- THEN a new UUID v4 is generated
- AND X-Request-ID header is added to the request context

### Scenario: Request ID Propagated
- GIVEN an incoming request with X-Request-ID: "abc-123"
- WHEN the request is processed
- THEN the same request ID is used in logs
- AND X-Request-ID: "abc-123" is returned in response headers

### Scenario: Request Logged
- GIVEN a request to GET /api/v1/users/me
- WHEN the request completes with status 200 in 45ms
- THEN log entry contains: {level: "INFO", method: "GET", path: "/api/v1/users/me", status: 200, duration_ms: 45, request_id: "uuid"}

### Scenario: JSON Log Format
- GIVEN any log output
- WHEN the log is written
- THEN output is valid JSON with structured fields

### Scenario: Error Logged
- GIVEN a request that results in 500 error
- WHEN the error is logged
- THEN log level is ERROR with error details
