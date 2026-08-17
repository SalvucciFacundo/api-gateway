# API Gateway

A self-contained Go HTTP gateway with JWT authentication, rate limiting, structured JSON logs, Prometheus metrics, health probes, and an embedded React frontend. The current store is in memory, so users and the seeded admin are recreated whenever the process starts.

## Quick Start

Requirements: Go 1.26.5 and Node.js with npm.

```bash
export JWT_SECRET='replace-with-a-long-random-secret'
export ADMIN_EMAIL='admin@example.com'
export ADMIN_PASSWORD='change-this-development-password'

cd web
npm ci
npm run build
cd ..
go run ./cmd/gateway
```

The gateway listens on `http://localhost:8080` by default. `PORT` can override the port.

## Architecture

```text
cmd/gateway
  configuration, admin seed, embedded filesystem, graceful shutdown
internal/server
  chi router and middleware/handler wiring
internal/middleware
  request IDs, JSON logging, Prometheus metrics, JWT auth, rate limiting
internal/service
  authentication and JWT business logic
internal/store
  in-memory user store
web
  React/Vite SPA; web/dist is embedded into the Go binary
```

Requests pass through request ID, structured logging, and metrics middleware. Authentication and rate limiting are applied to the relevant route groups. The server uses bounded read, write, idle, and shutdown timeouts.

## Environment Variables

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `JWT_SECRET` | Yes | None | HS256 signing secret. The process exits if it is empty. |
| `ADMIN_EMAIL` | No in Go; required by Compose | `admin@example.com` | Email for the idempotently seeded admin user. |
| `ADMIN_PASSWORD` | No in Go; required by Compose | `admin1234` | Password for the seeded admin user. |
| `PORT` | No | `8080` | TCP port on which the gateway listens. |

Set a strong, unique `JWT_SECRET` and admin password outside development. The Compose file requires all three credential variables so accidental default credentials are not used in a container.

## API Demo

The following is a complete local flow. It requires `jq` to extract the access token.

Register a user:

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"correct-horse"}'
```

Log in and retain the access token:

```bash
ACCESS_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"user@example.com","password":"correct-horse"}' | jq -r '.access_token')
```

Call protected endpoints:

```bash
curl -s http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer $ACCESS_TOKEN"

curl -s http://localhost:8080/api/v1/limits/status \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Refresh uses the `refresh_token` returned by login or refresh. Authentication errors use a JSON body such as `{"error":"invalid credentials"}`.

The seeded admin credentials are **development examples only**:

```text
Email:    admin@example.com
Password: admin1234
```

Do not use these values outside a local demonstration.

## Health, Readiness, And Metrics

| Endpoint | Auth | Purpose |
| --- | --- | --- |
| `GET /healthz` | No | Liveness probe; returns `{"status":"ok"}`. |
| `GET /readyz` | No | Readiness probe; returns `ready` or HTTP 503 with `not ready`. |
| `GET /metrics` | No | Prometheus exposition format. |

```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
curl -s http://localhost:8080/metrics
```

## Docker

The multi-stage Dockerfile builds the React assets, embeds them into a statically linked Go binary, and runs it as a non-root user. Compose exposes port 8080, validates the required credentials, uses a read-only root filesystem, and checks `/healthz`.

```bash
export JWT_SECRET='replace-with-a-long-random-secret'
export ADMIN_EMAIL='admin@example.com'
export ADMIN_PASSWORD='change-this-development-password'

docker compose up --build
```

Set `PORT` when another host/container port is needed. Compose maps and checks the selected port.

Stop the stack with:

```bash
docker compose down
```

## Frontend

The production frontend is built by `npm run build` and embedded from `web/dist`. Once the gateway is running, open `http://localhost:8080` to use the login, registration, and authenticated explorer pages.

The frontend stores access and refresh tokens in browser `localStorage` under `api-gateway.access-token` and `api-gateway.refresh-token`. This keeps the demo simple and allows the refresh flow, but any JavaScript executing in the page context can read those tokens if an XSS vulnerability exists. A production application should consider an HTTP-only, Secure, SameSite cookie strategy and a stronger content security policy, accepting the additional CSRF and session-management work that entails.

## Development Checks

```bash
go test ./...
cd web && npm run build
```

The Go tests cover the in-memory store, services, middleware, handlers, server wiring, and embedded frontend. The frontend build verifies TypeScript and Vite output.
