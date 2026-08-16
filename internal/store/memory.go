package store

import (
	"context"
	"errors"
	"sync"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
)

// Sentinel errors returned by MemoryStore.
var (
	// ErrEmailExists indicates a Create failed because the email is already
	// registered.
	ErrEmailExists = errors.New("store: email already exists")
	// ErrUserNotFound indicates a lookup found no matching user.
	ErrUserNotFound = errors.New("store: user not found")
	// ErrInvalidUser indicates a Create received a nil user or a user missing
	// its ID or email.
	ErrInvalidUser = errors.New("store: invalid user")
)

// MemoryStore is an in-memory Store implementation guarded by an RWMutex.
//
// users maps user ID to the User pointer; emails maps email to user ID so the
// uniqueness constraint is enforced with O(1) lookups. Both maps are protected
// by mu, making every operation safe for concurrent access.
type MemoryStore struct {
	mu     sync.RWMutex
	users  map[string]*model.User
	emails map[string]string
}

// Compile-time check that MemoryStore satisfies the Store interface.
var _ Store = (*MemoryStore)(nil)

// NewMemoryStore returns an empty, ready-to-use MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:  make(map[string]*model.User),
		emails: make(map[string]string),
	}
}

// Create stores user, failing with ErrInvalidUser for a nil or incomplete user
// and ErrEmailExists when the email is already taken.
func (s *MemoryStore) Create(ctx context.Context, user *model.User) error {
	if user == nil || user.ID == "" || user.Email == "" {
		return ErrInvalidUser
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.emails[user.Email]; exists {
		return ErrEmailExists
	}

	s.users[user.ID] = user
	s.emails[user.Email] = user.ID
	return nil
}

// GetByEmail returns the user with the given email, or ErrUserNotFound.
func (s *MemoryStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.emails[email]
	if !ok {
		return nil, ErrUserNotFound
	}
	user, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// GetByID returns the user with the given ID, or ErrUserNotFound.
func (s *MemoryStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}
