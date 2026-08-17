// Command gateway is the API Gateway entrypoint. It wires configuration, the
// in-memory store (with a seeded admin), the embedded SPA assets, and the HTTP
// server, then runs until SIGINT/SIGTERM with a graceful shutdown.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/SalvucciFacundo/api-gateway/internal/server"
	"github.com/SalvucciFacundo/api-gateway/internal/store"
	"github.com/SalvucciFacundo/api-gateway/web"
)

// config holds the runtime configuration resolved from the environment.
type config struct {
	Port          string
	JWTSecret     []byte
	AdminEmail    string
	AdminPassword string
	BcryptCost    int
}

// default values used when the corresponding environment variable is unset.
const (
	defaultPort          = "8080"
	defaultAdminEmail    = "admin@example.com"
	defaultAdminPassword = "admin1234"
	defaultBcryptCost    = 12
)

// errMissingJWTSecret is returned when JWT_SECRET is empty (JWT-AUTH-007).
var errMissingJWTSecret = errors.New("JWT_SECRET environment variable is required")

// loadConfig resolves the configuration from the environment via getenv. It
// fails fast when JWT_SECRET is missing or empty.
func loadConfig(getenv func(string) string) (config, error) {
	secret := getenv("JWT_SECRET")
	if secret == "" {
		return config{}, errMissingJWTSecret
	}

	port := getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	adminEmail := getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = defaultAdminEmail
	}
	adminPassword := getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = defaultAdminPassword
	}

	return config{
		Port:          port,
		JWTSecret:     []byte(secret),
		AdminEmail:    adminEmail,
		AdminPassword: adminPassword,
		BcryptCost:    defaultBcryptCost,
	}, nil
}

// staticFS returns the embedded frontend with its dist/ prefix stripped so the
// SPA handler serves files at the filesystem root.
func staticFS() (fs.FS, error) {
	return fs.Sub(web.Dist, "dist")
}

func main() {
	// LOG-001: all output is structured JSON.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("gateway exited with error", "error", err)
		os.Exit(1)
	}
}

// run wires and starts the gateway, blocking until the shutdown signal.
func run(logger *slog.Logger) error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}

	st := store.NewMemoryStore()
	if _, err := store.SeedAdmin(context.Background(), st, cfg.AdminEmail, cfg.AdminPassword, cfg.BcryptCost); err != nil {
		return err
	}

	assets, err := staticFS()
	if err != nil {
		return err
	}

	srv := server.NewServer(server.Config{
		Port:      cfg.Port,
		JWTSecret: cfg.JWTSecret,
		Logger:    logger,
		StaticFS:  assets,
		Store:     st,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("gateway starting", "port", cfg.Port)
	if err := srv.Start(ctx); err != nil {
		return err
	}
	logger.Info("server stopped gracefully")
	return nil
}
