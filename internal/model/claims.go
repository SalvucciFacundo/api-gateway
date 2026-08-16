package model

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload used for both access and refresh tokens.
//
// It embeds jwt.RegisteredClaims for the standard iss/sub/exp/iat fields and
// adds a Type discriminator so middleware can tell access tokens apart from
// refresh tokens.
type Claims struct {
	jwt.RegisteredClaims
	Type string `json:"type"` // "access" | "refresh"
}

// Token lifetimes and issuer shared by the auth service and middleware.
const (
	// AccessExpiration is the lifetime of an access token.
	AccessExpiration = 15 * time.Minute
	// RefreshExpiration is the lifetime of a refresh token.
	RefreshExpiration = 24 * time.Hour
	// Issuer identifies the gateway in the iss claim.
	Issuer = "api-gateway"
)
