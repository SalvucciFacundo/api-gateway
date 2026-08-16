// Package store defines the user persistence contract and its in-memory
// implementation.
package store

import (
	"context"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
)

// Store is the persistence contract for users.
//
// It is implemented today by MemoryStore and can be swapped for a
// Postgres-backed store without touching the service or handler layers.
type Store interface {
	Create(ctx context.Context, user *model.User) error
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id string) (*model.User, error)
}
