# Embedded Frontend Specification

## Purpose

React SPA (Vite) served via embed.FS from Go binary. Provides login/register UI and API explorer with real JWT claims, rate limit bars, and health status. Same-origin deployment eliminates CORS.

## Requirements

| ID | Requirement | Criteria |
|---|---|---|
| FE-001 | Build Output | Vite builds React SPA to web/dist/ directory |
| FE-002 | Embed FS | Go binary embeds web/dist/ using //go:embed |
| FE-003 | SPA Fallback | Non-/api/* and non-/metrics routes serve index.html |
| FE-004 | Static Assets | CSS, JS, images served with correct MIME types |
| FE-005 | Login Page | Login form with email/password, calls /api/v1/auth/login |
| FE-006 | Register Page | Register form with email/password, calls /api/v1/auth/register |
| FE-007 | API Explorer | Displays JWT token claims (decoded from access token) |
| FE-008 | Rate Limit Bars | Visual bars showing per-bucket usage from /api/v1/limits/status |
| FE-009 | Health Status | Displays /healthz status (ok/not ready) |
| FE-010 | Same Origin | Frontend served from same origin as API (no CORS) |
| FE-011 | No Demo Data | Frontend MUST display real API data, not mock/demo data |

## Scenarios

### Scenario: Serve Frontend Root
- GIVEN the Go binary is running
- WHEN GET / is called
- THEN index.html is served from embedded FS

### Scenario: Serve Static Asset
- GIVEN a request for /assets/main.js
- WHEN the request is made
- THEN the JS file is served with correct Content-Type

### Scenario: SPA Fallback
- GIVEN a request for /dashboard (non-API route)
- WHEN the request is made
- THEN index.html is served (SPA handles routing)

### Scenario: API Route Not Caught
- GIVEN a request for /api/v1/users/me
- WHEN the request is made
- THEN the request is NOT caught by SPA fallback
- AND reaches the API handler

### Scenario: Login Form Submission
- GIVEN a user on the login page
- WHEN the form is submitted with valid credentials
- THEN /api/v1/auth/login is called
- AND access token is stored

### Scenario: API Explorer Shows Claims
- GIVEN a logged-in user with valid access token
- WHEN the API explorer page is viewed
- THEN JWT claims (sub, exp, iat, type) are displayed

### Scenario: Rate Limit Bars Display
- GIVEN an authenticated user
- WHEN /api/v1/limits/status returns {ip: {limit: 100, remaining: 50}}
- THEN a visual bar shows 50% usage

### Scenario: Health Status Display
- GIVEN the application is healthy
- WHEN the health status component loads
- THEN it displays "ok" status from /healthz

### Scenario: No Demo Data
- GIVEN the frontend is loaded
- WHEN no API calls have been made
- THEN no mock/demo data is displayed
- AND UI shows empty/loading state
