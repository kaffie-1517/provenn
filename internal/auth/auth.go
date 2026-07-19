// Package auth handles JWT issue/verify, API-key middleware, password hashing,
// and the context helpers that enforce the tenancy rule (LLD §6).
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidCredentials is returned when email/password don't match.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidToken is returned when a JWT is malformed or expired.
	ErrInvalidToken = errors.New("invalid or expired token")
)

// Claims carried in every ProveNN JWT.
// company_id is nil for platform_admin — all other roles have it set.
type Claims struct {
	jwt.RegisteredClaims
	UserID    uuid.UUID  `json:"user_id"`
	Role      string     `json:"role"`
	CompanyID *uuid.UUID `json:"company_id"`
}

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(h), err
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// IssueJWT creates an HS256-signed JWT containing user_id, role, and
// company_id (nil for platform_admin). Tokens expire after 24 hours.
func IssueJWT(secret string, userID uuid.UUID, role string, companyID *uuid.UUID) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			Issuer:    "provenn",
		},
		UserID:    userID,
		Role:      role,
		CompanyID: companyID,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseJWT validates the token signature and expiry, returning the claims.
func ParseJWT(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
