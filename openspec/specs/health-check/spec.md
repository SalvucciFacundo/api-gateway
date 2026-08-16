# Health Check Specification

## Purpose

Liveness and readiness probes for container orchestration. Liveness returns 200 OK; readiness checks store availability.

## Requirements

| ID | Requirement | Criteria |
|---|---|---|
| HEALTH-001 | Liveness Probe | GET /healthz returns 200 OK with {status: "ok"} |
| HEALTH-002 | Readiness Check | Readiness probe checks store availability |
| HEALTH-003 | Readiness Response | Returns 200 if store is ready, 503 if not ready |
| HEALTH-004 | No Auth Required | /healthz endpoint MUST NOT require authentication |

## Scenarios

### Scenario: Liveness Probe Success
- GIVEN the application is running
- WHEN GET /healthz is called
- THEN response is 200 OK with {status: "ok"}

### Scenario: Readiness Probe Success
- GIVEN the store is initialized and accessible
- WHEN readiness check is performed
- THEN response is 200 OK with {status: "ready"}

### Scenario: Readiness Probe Failure
- GIVEN the store is unavailable or not initialized
- WHEN readiness check is performed
- THEN response is 503 Service Unavailable with {status: "not ready"}

### Scenario: Health Check Without Auth
- GIVEN the application is running
- WHEN GET /healthz is called without authentication
- THEN response is 200 OK (no 401)
