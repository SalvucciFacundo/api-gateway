package model

import (
	"testing"
	"time"
)

// TestUserFields verifies STORE-003: User carries ID (UUID), Email,
// PasswordHash, and CreatedAt as the store layer expects.
func TestUserFields(t *testing.T) {
	created := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)

	u := User{
		ID:           "123e4567-e89b-12d3-a456-426614174000",
		Email:        "user@example.com",
		PasswordHash: "$2a$12$abcdefghijklmnopqrstuv",
		CreatedAt:    created,
	}

	if u.ID != "123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("ID = %q, want the provided UUID", u.ID)
	}
	if u.Email != "user@example.com" {
		t.Errorf("Email = %q, want user@example.com", u.Email)
	}
	if u.PasswordHash == "" {
		t.Error("PasswordHash = empty, want the provided hash")
	}
	if !u.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v", u.CreatedAt, created)
	}
}
