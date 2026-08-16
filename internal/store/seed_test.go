package store

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestSeedAdminCreatesAdmin verifies STORE-004: seeding produces an admin user
// with a generated UUID and a bcrypt-hashed password (never plaintext).
func TestSeedAdminCreatesAdmin(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	user, err := SeedAdmin(ctx, s, "admin@example.com", "admin1234", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("SeedAdmin() error = %v", err)
	}
	if user == nil {
		t.Fatal("SeedAdmin() returned nil user")
	}
	if user.Email != "admin@example.com" {
		t.Errorf("Email = %q, want admin@example.com", user.Email)
	}
	if user.ID == "" {
		t.Error("ID = empty, want a generated UUID")
	}
	if user.PasswordHash == "admin1234" {
		t.Error("PasswordHash = plaintext password, want a bcrypt hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("admin1234")); err != nil {
		t.Errorf("PasswordHash does not match password: %v", err)
	}
}

// TestSeedAdminIdempotent verifies STORE-004: seeding twice returns the same
// user and does not create a duplicate.
func TestSeedAdminIdempotent(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	first, err := SeedAdmin(ctx, s, "admin@example.com", "admin1234", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("first SeedAdmin() error = %v", err)
	}

	second, err := SeedAdmin(ctx, s, "admin@example.com", "admin1234", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("second SeedAdmin() error = %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("second SeedAdmin() ID = %q, want %q (idempotent)", second.ID, first.ID)
	}

	got, err := s.GetByEmail(ctx, "admin@example.com")
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if got.ID != first.ID {
		t.Errorf("GetByEmail().ID = %q, want %q", got.ID, first.ID)
	}
}
