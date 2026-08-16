# Delta for Prometheus Metrics

## ADDED Requirements

| ID | Requirement | Criteria |
|---|---|---|
| METRICS-001 | Request Counter | Counter vector with labels: method, path, status |
| METRICS-002 | Latency Histogram | Histogram vector with labels: method, path; buckets: 10ms, 50ms, 100ms, 500ms, 1s, 5s |
| METRICS-003 | Metrics Endpoint | GET /metrics returns Prometheus-formatted metrics |
| METRICS-004 | No Auth Required | /metrics endpoint MUST NOT require authentication |
| METRICS-005 | Middleware Integration | Metrics middleware records every request automatically |

## Scenarios

### Scenario: Metrics Endpoint Accessible
- GIVEN the application is running
- WHEN GET /metrics is called without authentication
- THEN response is 200 OK with Prometheus text format

### Scenario: Request Counter Incremented
- GIVEN a request to GET /api/v1/users/me completes with 200
- WHEN /metrics is scraped
- THEN http_requests_total{method="GET",path="/api/v1/users/me",status="200"} is incremented

### Scenario: Latency Histogram Recorded
- GIVEN a request completes in 45ms
- WHEN /metrics is scraped
- THEN http_request_duration_seconds_bucket{le="0.05"} is incremented

### Scenario: Multiple Status Codes Tracked
- GIVEN requests resulting in 200, 401, and 500 status codes
- WHEN /metrics is scraped
- THEN counters show separate entries for each status code
