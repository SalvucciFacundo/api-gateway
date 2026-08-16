# User Store Specification

## Purpose

In-memory user persistence with swappable Store interface. Provides Create, GetByEmail, GetByID operations. Seeds admin user on startup.

## Requirements

| ID | Requirement | Criteria |
|---|---|---|
| STORE-001 | Store Interface | Interface defines: Create(user) error, GetByEmail(email) (*User, error), GetByID(id) (*User, error) |
| STORE-002 | MemoryStore Implementation | In-memory implementation using sync.RWMutex for thread safety |
| STORE-003 | User Model | User struct with: ID (UUID), Email, PasswordHash, CreatedAt |
| STORE-004 | Admin Seed | On startup, create admin user if not exists (email from env or default) |
| STORE-005 | Email Uniqueness | GetByEmail returns error if email not found; Create fails if email exists |
| STORE-006 | Thread Safety | All operations MUST be safe for concurrent access |

## Scenarios

### Scenario: Create User
- GIVEN a new user with email "user@example.com"
- WHEN Create(user) is called
- THEN user is stored with generated UUID
- AND no error is returned

### Scenario: Create Duplicate Email
- GIVEN an existing user with email "user@example.com"
- WHEN Create(user) is called with same email
- THEN error is returned (duplicate email)

### Scenario: Get User By Email
- GIVEN a user with email "user@example.com" exists
- WHEN GetByEmail("user@example.com") is called
- THEN user is returned with all fields populated

### Scenario: Get Non-Existent User
- GIVEN no user with email "missing@example.com"
- WHEN GetByEmail("missing@example.com") is called
- THEN error is returned (user not found)

### Scenario: Get User By ID
- GIVEN a user with ID "uuid-123" exists
- WHEN GetByID("uuid-123") is called
- THEN user is returned

### Scenario: Admin User Seeded
- GIVEN the application starts for the first time
- WHEN initialization completes
- THEN admin user exists in store

### Scenario: Concurrent Access
- GIVEN multiple goroutines accessing the store
- WHEN Create and Get operations happen concurrently
- THEN no race conditions occur
- AND all operations complete successfully
