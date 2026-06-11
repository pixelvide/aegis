// Package auth provides password hashing and JWT token management.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Service manages authentication operations.
type Service struct {
	jwtSecret []byte
	tokenTTL  time.Duration
}

// New creates an auth Service. Reads JWT_SECRET from env.
// In development, auto-generates a random secret if not set.
func New() (*Service, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Auto-generate for development only
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generate jwt secret: %w", err)
		}
		secret = hex.EncodeToString(b)
		fmt.Println("⚠️  JWT_SECRET not set — generated ephemeral secret (sessions won't survive restart)")
	}

	return &Service{
		jwtSecret: []byte(secret),
		tokenTTL:  24 * time.Hour,
	}, nil
}

// ─── Password Hashing ───────────────────────────────────────────────────────

// HashPassword hashes a plaintext password using bcrypt (cost 12).
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), 12)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a bcrypt hash with a plaintext password.
// Returns nil if they match.
func CheckPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

// ─── JWT ─────────────────────────────────────────────────────────────────────

// Claims is the JWT payload.
type Claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT for the given user ID.
func (s *Service) GenerateToken(userID string) (string, error) {
	now := time.Now().UTC()
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenTTL)),
			Issuer:    "aegis",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateToken parses and validates a JWT, returning the user ID.
func (s *Service) ValidateToken(tokenStr string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Verify signing method to prevent algorithm switching attacks
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	if claims.UserID == "" {
		return "", fmt.Errorf("token missing user ID")
	}
	return claims.UserID, nil
}
