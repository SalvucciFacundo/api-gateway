// Package service holds the business logic for authentication.
package service

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/SalvucciFacundo/api-gateway/internal/model"
	"github.com/SalvucciFacundo/api-gateway/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Sentinel errors returned by AuthService.
var (
	// ErrInvalidEmail indicates a registration email failed format validation.
	ErrInvalidEmail = errors.New("service: invalid email")
	// ErrWeakPassword indicates a password did not meet the minimum length.
	ErrWeakPassword = errors.New("service: weak password")
	// ErrEmailExists indicates a registration collided with an existing email.
	ErrEmailExists = errors.New("service: email already exists")
	// ErrInvalidCredentials indicates login failed (wrong email or password).
	ErrInvalidCredentials = errors.New("service: invalid credentials")
	// ErrInvalidToken indicates a token failed signature/expiry/issuer/type
	// validation.
	ErrInvalidToken = errors.New("service: invalid token")
)

const (
	// defaultBcryptCost is the bcrypt work factor used for new passwords.
	defaultBcryptCost = 12
	// minPasswordLen is the minimum accepted password length.
	minPasswordLen = 8
)

// tokenType values used as the Type discriminator in JWT claims.
const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"
)

// emailPattern is a deliberately simple format check: local@domain.tld with no
// whitespace. It is not a full RFC 5322 validator, which is out of scope for a
// demo gateway.
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// AuthService implements registration, login, refresh, and user lookup.
type AuthService struct {
	store      store.Store
	jwtSecret  []byte
	bcryptCost int
}

// NewAuthService returns an AuthService backed by s, signing tokens with
// jwtSecret and hashing passwords with bcrypt at cost 12.
func NewAuthService(s store.Store, jwtSecret []byte) *AuthService {
	return &AuthService{
		store:      s,
		jwtSecret:  jwtSecret,
		bcryptCost: defaultBcryptCost,
	}
}

// Register validates the request, hashes the password, and stores a new user.
func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	if !emailPattern.MatchString(req.Email) {
		return nil, ErrInvalidEmail
	}
	if len(req.Password) < minPasswordLen {
		return nil, ErrWeakPassword
	}

	if _, err := s.store.GetByEmail(ctx, req.Email); err == nil {
		return nil, ErrEmailExists
	} else if !errors.Is(err, store.ErrUserNotFound) {
		return nil, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.bcryptCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Email:        req.Email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}

	if err := s.store.Create(ctx, user); err != nil {
		if errors.Is(err, store.ErrEmailExists) {
			return nil, ErrEmailExists
		}
		return nil, err
	}

	return user, nil
}

// Login verifies the credentials and returns a fresh access/refresh pair.
func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, error) {
	user, err := s.store.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.generateTokenPair(user.ID)
}

// Refresh validates a refresh token and returns a fresh token pair for its
// subject.
func (s *AuthService) Refresh(ctx context.Context, req model.RefreshRequest) (*model.AuthResponse, error) {
	claims, err := s.parseToken(req.RefreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Type != tokenTypeRefresh {
		return nil, ErrInvalidToken
	}

	return s.generateTokenPair(claims.Subject)
}

// GetUser returns the user with the given ID.
func (s *AuthService) GetUser(ctx context.Context, userID string) (*model.User, error) {
	return s.store.GetByID(ctx, userID)
}

// generateTokenPair issues a new access and refresh token for userID.
func (s *AuthService) generateTokenPair(userID string) (*model.AuthResponse, error) {
	accessToken, err := s.generateToken(userID, tokenTypeAccess, model.AccessExpiration)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateToken(userID, tokenTypeRefresh, model.RefreshExpiration)
	if err != nil {
		return nil, err
	}

	return &model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// generateToken signs an HS256 JWT carrying iss/sub/exp/iat and a type
// discriminator.
func (s *AuthService) generateToken(userID, tokenType string, duration time.Duration) (string, error) {
	now := time.Now()
	claims := &model.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    model.Issuer,
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Type: tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// parseToken validates a token's signature, expiry, and issuer, returning its
// claims. It deliberately does not check the type discriminator, so callers can
// enforce the access/refresh distinction appropriate to their flow.
func (s *AuthService) parseToken(tokenString string) (*model.Claims, error) {
	claims := &model.Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	if claims.Issuer != model.Issuer {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
