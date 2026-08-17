package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// newPostgresStore connects to the TEST_DATABASE_URL database and applies the
// embedded migrations. It skips the test when the variable is unset so the
// default test suite needs no running database.
func newPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres store integration test")
	}
	ctx := context.Background()
	st, err := NewPostgresStore(ctx, url)
	if err != nil {
		t.Fatalf("NewPostgresStore: %v", err)
	}
	t.Cleanup(st.Close)
	return st
}

// postgresTestUser builds a user with a unique ID and email so tests stay
// isolated and can re-run against the same database.
func postgresTestUser(t *testing.T) *model.User {
	t.Helper()
	return &model.User{
		ID:           uuid.NewString(),
		Email:        "user-" + uuid.NewString() + "@example.com",
		PasswordHash: "bcrypt-placeholder",
		CreatedAt:    time.Now(),
	}
}

// TestPostgresStoreCreateAndGet verifies the Postgres store round-trips a user
// through Create, GetByID and GetByEmail with all fields intact.
func TestPostgresStoreCreateAndGet(t *testing.T) {
	s := newPostgresStore(t)
	ctx := context.Background()
	u := postgresTestUser(t)

	if err := s.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	byID, err := s.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if byID.Email != u.Email || byID.PasswordHash != u.PasswordHash || byID.CreatedAt.IsZero() {
		t.Errorf("GetByID() = %+v, want email %q with hash and created_at", byID, u.Email)
	}

	byEmail, err := s.GetByEmail(ctx, u.Email)
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if byEmail.ID != u.ID {
		t.Errorf("GetByEmail().ID = %q, want %q", byEmail.ID, u.ID)
	}
}

// TestPostgresStoreDuplicateEmail verifies a second Create with the same email
// is rejected with ErrEmailExists, mirroring STORE-005.
func TestPostgresStoreDuplicateEmail(t *testing.T) {
	s := newPostgresStore(t)
	ctx := context.Background()
	u := postgresTestUser(t)

	if err := s.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	dup := &model.User{ID: uuid.NewString(), Email: u.Email, PasswordHash: "other", CreatedAt: time.Now()}
	if err := s.Create(ctx, dup); !errors.Is(err, ErrEmailExists) {
		t.Fatalf("Create() duplicate error = %v, want ErrEmailExists", err)
	}
}

// TestPostgresStoreNotFound verifies missing lookups return ErrUserNotFound.
func TestPostgresStoreNotFound(t *testing.T) {
	s := newPostgresStore(t)
	ctx := context.Background()

	if _, err := s.GetByEmail(ctx, "missing@example.com"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("GetByEmail() error = %v, want ErrUserNotFound", err)
	}
	if _, err := s.GetByID(ctx, "missing-id"); !errors.Is(err, ErrUserNotFound) {
		t.Errorf("GetByID() error = %v, want ErrUserNotFound", err)
	}
}

// TestPostgresStoreInvalidUser verifies Create rejects nil or incomplete users
// instead of inserting partial rows.
func TestPostgresStoreInvalidUser(t *testing.T) {
	s := newPostgresStore(t)
	ctx := context.Background()

	if err := s.Create(ctx, nil); !errors.Is(err, ErrInvalidUser) {
		t.Errorf("Create(nil) error = %v, want ErrInvalidUser", err)
	}
	if err := s.Create(ctx, &model.User{Email: "a@example.com"}); !errors.Is(err, ErrInvalidUser) {
		t.Errorf("Create(missing id) error = %v, want ErrInvalidUser", err)
	}
	if err := s.Create(ctx, &model.User{ID: uuid.NewString()}); !errors.Is(err, ErrInvalidUser) {
		t.Errorf("Create(missing email) error = %v, want ErrInvalidUser", err)
	}
}

// TestPostgresStorePing verifies Ping succeeds against a reachable database.
func TestPostgresStorePing(t *testing.T) {
	s := newPostgresStore(t)
	if err := s.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

// TestPostgresStoreSeedAdmin verifies the existing SeedAdmin flow is idempotent
// against the Postgres store.
func TestPostgresStoreSeedAdmin(t *testing.T) {
	s := newPostgresStore(t)
	ctx := context.Background()
	email := "admin-" + uuid.NewString() + "@example.com"

	first, err := SeedAdmin(ctx, s, email, "admin1234", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("first SeedAdmin() error = %v", err)
	}

	second, err := SeedAdmin(ctx, s, email, "admin1234", bcrypt.MinCost)
	if err != nil {
		t.Fatalf("second SeedAdmin() error = %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("second SeedAdmin() ID = %q, want %q (idempotent)", second.ID, first.ID)
	}
}
