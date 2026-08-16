# JWT Authentication Specification

## Purpose

JWT-based authentication system with registration, login, token refresh, and middleware validation. Uses HS256 signing with claims: iss, sub, exp, iat, type (access|refresh).

## Requirements

| ID | Requirement | Criteria |
|---|---|---|
| JWT-AUTH-001 | User Registration | POST /api/v1/auth/register accepts {email, password}, creates user with bcrypt-hashed password, returns 201 |
| JWT-AUTH-002 | User Login | POST /api/v1/auth/login accepts {email, password}, validates credentials, returns {access_token, refresh_token} with 200 |
| JWT-AUTH-003 | Token Refresh | POST /api/v1/auth/refresh accepts {refresh_token}, validates signature/exp, returns new {access_token, refresh_token} |
| JWT-AUTH-004 | JWT Middleware | Validates signature, exp, iss claims; extracts sub (user ID) and type; rejects invalid/expired tokens with 401 |
| JWT-AUTH-005 | Protected Endpoints | GET /api/v1/users/me requires valid access token in Authorization header, returns user data |
| JWT-AUTH-006 | JWT Claims | Tokens MUST contain: iss (issuer), sub (user ID), exp (expiration), iat (issued at), type (access\|refresh) |
| JWT-AUTH-007 | JWT Secret | MUST read JWT_SECRET from environment; fail-fast on startup if missing |
| JWT-AUTH-008 | Token Expiration | Access tokens expire in 15 minutes; refresh tokens expire in 24 hours |

## Scenarios

### Scenario: Successful Registration
- GIVEN a new user with valid email and password
- WHEN POST /api/v1/auth/register is called with {email: "user@example.com", password: "securePass123"}
- THEN response is 201 Created with {id, email}
- AND password is stored as bcrypt hash

### Scenario: Duplicate Email Registration
- GIVEN an existing user with email "user@example.com"
- WHEN POST /api/v1/auth/register is called with same email
- THEN response is 409 Conflict with error message

### Scenario: Successful Login
- GIVEN a registered user with email "user@example.com"
- WHEN POST /api/v1/auth/login is called with valid credentials
- THEN response is 200 OK with {access_token, refresh_token}
- AND access_token expires in 15 minutes
- AND refresh_token expires in 24 hours

### Scenario: Invalid Credentials Login
- GIVEN a registered user
- WHEN POST /api/v1/auth/login is called with wrong password
- THEN response is 401 Unauthorized

### Scenario: Valid Token Refresh
- GIVEN a valid refresh token
- WHEN POST /api/v1/auth/refresh is called with {refresh_token}
- THEN response is 200 OK with new {access_token, refresh_token}

### Scenario: Expired Refresh Token
- GIVEN an expired refresh token
- WHEN POST /api/v1/auth/refresh is called
- THEN response is 401 Unauthorized

### Scenario: Access Protected Endpoint
- GIVEN a valid access token
- WHEN GET /api/v1/users/me is called with Authorization: Bearer <token>
- THEN response is 200 OK with {id, email}

### Scenario: Invalid Access Token
- GIVEN an invalid or expired access token
- WHEN GET /api/v1/users/me is called
- THEN response is 401 Unauthorized

### Scenario: Missing JWT_SECRET
- GIVEN JWT_SECRET environment variable is not set
- WHEN the application starts
- THEN the application MUST exit with error message
