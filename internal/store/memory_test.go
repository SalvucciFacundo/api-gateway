package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
)

func testUser(id, email string) *model.User {
	return &model.User{
		ID:        id,
		Email:     email,
		CreatedAt: time.Now(),
	}
}

// TestMemoryStoreCreateAndGet verifies STORE-001: Create stores a user that is
// retrievable by both ID and email with all fields intact.
func TestMemoryStoreCreateAndGet(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()
	u := testUser("uuid-1", "user@example.com")

	if err := s.Create(ctx, u); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	byID, err := s.GetByID(ctx, "uuid-1")
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if byID.Email != "user@example.com" {
		t.Errorf("GetByID().Email = %q, want user@example.com", byID.Email)
	}

	byEmail, err := s.GetByEmail(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("GetByEmail() error = %v", err)
	}
	if byEmail.ID != "uuid-1" {
		t.Errorf("GetByEmail().ID = %q, want uuid-1", byEmail.ID)
	}
}

// TestMemoryStoreDuplicateEmail verifies STORE-005: a second Create with the
// same email is rejected with ErrEmailExists.
func TestMemoryStoreDuplicateEmail(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Create(ctx, testUser("uuid-1", "dup@example.com")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err := s.Create(ctx, testUser("uuid-2", "dup@example.com"))
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("Create() duplicate error = %v, want ErrEmailExists", err)
	}
}

// TestMemoryStoreNotFound verifies missing lookups return ErrUserNotFound.
func TestMemoryStoreNotFound(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	t.Run("email", func(t *testing.T) {
		if _, err := s.GetByEmail(ctx, "missing@example.com"); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("GetByEmail() error = %v, want ErrUserNotFound", err)
		}
	})
	t.Run("id", func(t *testing.T) {
		if _, err := s.GetByID(ctx, "missing-id"); !errors.Is(err, ErrUserNotFound) {
			t.Errorf("GetByID() error = %v, want ErrUserNotFound", err)
		}
	})
}

// TestMemoryStoreInvalidUser verifies Create rejects nil or incomplete users
// instead of panicking.
func TestMemoryStoreInvalidUser(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	t.Run("nil user", func(t *testing.T) {
		if err := s.Create(ctx, nil); !errors.Is(err, ErrInvalidUser) {
			t.Errorf("Create(nil) error = %v, want ErrInvalidUser", err)
		}
	})
	t.Run("missing id", func(t *testing.T) {
		if err := s.Create(ctx, &model.User{Email: "a@example.com"}); !errors.Is(err, ErrInvalidUser) {
			t.Errorf("Create(missing id) error = %v, want ErrInvalidUser", err)
		}
	})
	t.Run("missing email", func(t *testing.T) {
		if err := s.Create(ctx, &model.User{ID: "uuid-1"}); !errors.Is(err, ErrInvalidUser) {
			t.Errorf("Create(missing email) error = %v, want ErrInvalidUser", err)
		}
	})
}

// TestMemoryStoreConcurrentAccess verifies STORE-006: many goroutines can
// create and read without races or data loss (run with -race).
func TestMemoryStoreConcurrentAccess(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	const goroutines = 50
	const perGoroutine = 20

	// Phase 1: concurrent creates with unique emails.
	var createWG sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		createWG.Add(1)
		go func(n int) {
			defer createWG.Done()
			for j := 0; j < perGoroutine; j++ {
				u := testUser(fmt.Sprintf("id-%d-%d", n, j), fmt.Sprintf("user-%d-%d@example.com", n, j))
				if err := s.Create(ctx, u); err != nil {
					t.Errorf("Create() error = %v", err)
				}
			}
		}(i)
	}
	createWG.Wait()

	// Phase 2: concurrent reads of everything created.
	var readWG sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		readWG.Add(1)
		go func(n int) {
			defer readWG.Done()
			for j := 0; j < perGoroutine; j++ {
				email := fmt.Sprintf("user-%d-%d@example.com", n, j)
				u, err := s.GetByEmail(ctx, email)
				if err != nil {
					t.Errorf("GetByEmail(%q) error = %v", email, err)
					continue
				}
				if u.Email != email {
					t.Errorf("GetByEmail(%q).Email = %q, want %q", email, u.Email, email)
				}
			}
		}(i)
	}
	readWG.Wait()
}

// TestMemoryStoreConcurrentMixedAccess exercises simultaneous reads and writes
// to verify the RWMutex holds under contention (run with -race).
func TestMemoryStoreConcurrentMixedAccess(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	if err := s.Create(ctx, testUser("seed", "seed@example.com")); err != nil {
		t.Fatalf("seed Create() error = %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			u := testUser(fmt.Sprintf("id-%d", n), fmt.Sprintf("user-%d@example.com", n))
			if err := s.Create(ctx, u); err != nil {
				t.Errorf("Create() error = %v", err)
			}
		}(i)
		go func() {
			defer wg.Done()
			if _, err := s.GetByEmail(ctx, "seed@example.com"); err != nil {
				t.Errorf("GetByEmail(seed) error = %v", err)
			}
		}()
	}
	wg.Wait()
}

// TestMemoryStoreConcurrentUniqueEmail verifies that when many goroutines race
// to create the same email, exactly one succeeds.
func TestMemoryStoreConcurrentUniqueEmail(t *testing.T) {
	s := NewMemoryStore()
	ctx := context.Background()

	const goroutines = 50
	var wg sync.WaitGroup
	var success int32

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			u := testUser(fmt.Sprintf("id-%d", n), "same@example.com")
			if err := s.Create(ctx, u); err == nil {
				atomic.AddInt32(&success, 1)
			}
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&success); got != 1 {
		t.Errorf("successful creates = %d, want exactly 1", got)
	}
}
