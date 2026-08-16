package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestTokenExpirations verifies JWT-AUTH-008: access tokens last 15 minutes
// and refresh tokens last 24 hours, with a fixed issuer.
func TestTokenExpirations(t *testing.T) {
	if AccessExpiration != 15*time.Minute {
		t.Errorf("AccessExpiration = %v, want 15m", AccessExpiration)
	}
	if RefreshExpiration != 24*time.Hour {
		t.Errorf("RefreshExpiration = %v, want 24h", RefreshExpiration)
	}
	if Issuer != "api-gateway" {
		t.Errorf("Issuer = %q, want api-gateway", Issuer)
	}
}

// TestClaimsJSONFields verifies JWT-AUTH-006: a marshaled token carries the
// standard iss/sub/exp/iat claims plus a type discriminator.
func TestClaimsJSONFields(t *testing.T) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessExpiration)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Type: "access",
	}

	data, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	for _, field := range []string{"iss", "sub", "exp", "iat", "type"} {
		if _, ok := got[field]; !ok {
			t.Errorf("marshaled Claims missing %q field: %s", field, data)
		}
	}
	if got["type"] != "access" {
		t.Errorf("type = %v, want access", got["type"])
	}
	if got["iss"] != "api-gateway" {
		t.Errorf("iss = %v, want api-gateway", got["iss"])
	}
	if got["sub"] != "user-123" {
		t.Errorf("sub = %v, want user-123", got["sub"])
	}
}
