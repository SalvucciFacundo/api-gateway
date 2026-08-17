package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// migrationsFS holds the embedded goose migration files applied automatically
// at startup, so a single container needs no separate migration job.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// PostgresStore is a Store backed by PostgreSQL via a pgx connection pool.
// It mirrors the MemoryStore contract exactly: the same sentinel errors
// (ErrEmailExists, ErrUserNotFound, ErrInvalidUser) and the same method set.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// Compile-time check that PostgresStore satisfies the Store interface.
var _ Store = (*PostgresStore)(nil)

// NewPostgresStore connects to the database at url, applies the embedded
// goose migrations, and returns a ready store. The caller owns the store and
// should Close it when done.
func NewPostgresStore(ctx context.Context, url string) (*PostgresStore, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging postgres: %w", err)
	}
	if err := migrate(ctx, url); err != nil {
		pool.Close()
		return nil, fmt.Errorf("applying migrations: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Close releases the connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// migrate applies the embedded goose migrations to the database at url.
// Migrations run through the pgx stdlib driver because the current goose
// provider API is database/sql based; the store itself keeps the pgxpool.
func migrate(ctx context.Context, url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationsFS)
	if err != nil {
		return err
	}
	if _, err := provider.Up(ctx); err != nil {
		return err
	}
	return nil
}

// Create inserts user, translating the unique-constraint violation on email
// into ErrEmailExists to match the MemoryStore contract.
func (s *PostgresStore) Create(ctx context.Context, user *model.User) error {
	if user == nil || user.ID == "" || user.Email == "" {
		return ErrInvalidUser
	}

	createdAt := user.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, created_at) VALUES ($1, $2, $3, $4)`,
		user.ID, user.Email, user.PasswordHash, createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrEmailExists
		}
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// GetByEmail returns the user with the given email, or ErrUserNotFound.
func (s *PostgresStore) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.scanUser(ctx, `SELECT id, email, password_hash, created_at FROM users WHERE email = $1`, email)
}

// GetByID returns the user with the given ID, or ErrUserNotFound.
func (s *PostgresStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	return s.scanUser(ctx, `SELECT id, email, password_hash, created_at FROM users WHERE id = $1`, id)
}

// Ping reports whether the database is reachable, backing the readiness probe
// (HEALTH-002/003).
func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// scanUser reads one users row in the column order of the SELECT statements
// above.
func (s *PostgresStore) scanUser(ctx context.Context, query, arg string) (*model.User, error) {
	var u model.User
	err := s.pool.QueryRow(ctx, query, arg).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("querying user: %w", err)
	}
	return &u, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505), e.g. a duplicate users.email.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
