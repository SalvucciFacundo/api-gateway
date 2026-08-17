package main

import "testing"

// envMap builds a getenv func from a map, returning "" for missing keys.
func envMap(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

// TestLoadConfigMissingJWTSecret verifies JWT-AUTH-007: an empty JWT_SECRET
// makes loadConfig fail fast with an error.
func TestLoadConfigMissingJWTSecret(t *testing.T) {
	_, err := loadConfig(envMap(map[string]string{}))
	if err == nil {
		t.Fatal("loadConfig returned nil error, want fail-fast on missing JWT_SECRET")
	}
}

// TestLoadConfigReadsValues verifies loadConfig reads JWT_SECRET and the
// optional PORT/ADMIN_EMAIL/ADMIN_PASSWORD values from the environment.
func TestLoadConfigReadsValues(t *testing.T) {
	cfg, err := loadConfig(envMap(map[string]string{
		"JWT_SECRET":     "super-secret",
		"PORT":           "9090",
		"ADMIN_EMAIL":    "admin@example.org",
		"ADMIN_PASSWORD": "s3cret-pass",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if string(cfg.JWTSecret) != "super-secret" {
		t.Errorf("JWTSecret = %q, want super-secret", cfg.JWTSecret)
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want 9090", cfg.Port)
	}
	if cfg.AdminEmail != "admin@example.org" {
		t.Errorf("AdminEmail = %q, want admin@example.org", cfg.AdminEmail)
	}
	if cfg.AdminPassword != "s3cret-pass" {
		t.Errorf("AdminPassword = %q, want s3cret-pass", cfg.AdminPassword)
	}
}

// TestLoadConfigDefaults verifies loadConfig applies demo defaults when the
// optional variables are absent.
func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := loadConfig(envMap(map[string]string{"JWT_SECRET": "secret"}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default 8080", cfg.Port)
	}
	if cfg.AdminEmail != "admin@example.com" {
		t.Errorf("AdminEmail = %q, want default admin@example.com", cfg.AdminEmail)
	}
	if cfg.AdminPassword != "admin1234" {
		t.Errorf("AdminPassword = %q, want default admin1234", cfg.AdminPassword)
	}
	if cfg.BcryptCost != 12 {
		t.Errorf("BcryptCost = %d, want 12", cfg.BcryptCost)
	}
}
