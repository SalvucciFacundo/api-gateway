package store

import (
	"context"
	"errors"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// SeedAdmin creates the admin user if it does not already exist.
//
// It is idempotent: when the admin already exists, it returns that user
// unchanged. Otherwise it hashes the password with bcrypt at the given cost
// and stores a fresh user with a generated UUID.
func SeedAdmin(ctx context.Context, s Store, email, password string, bcryptCost int) (*model.User, error) {
	existing, err := s.GetByEmail(ctx, email)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrUserNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}
	if err := s.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}
