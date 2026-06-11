// Package auth provides password hashing and JWT token management.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
	UserID     string `json:"uid"`
	JTI        string `json:"jti,omitempty"`        // session ID for revocation
	MFAPending bool   `json:"mfa_pending,omitempty"` // true = password verified, MFA not yet completed
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT for the given user ID.
// Returns the token string and the JTI (for session tracking).
func (s *Service) GenerateToken(userID string) (string, string, error) {
	now := time.Now().UTC()
	jti := uuid.New().String()
	claims := &Claims{
		UserID: userID,
		JTI:    jti,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.tokenTTL)),
			Issuer:    "aegis",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", "", err
	}
	return signed, jti, nil
}

// ValidateToken parses and validates a JWT, returning the user ID and JTI.
func (s *Service) ValidateToken(tokenStr string) (string, string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Verify signing method to prevent algorithm switching attacks
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", "", fmt.Errorf("parse token: %w", err)
	}
	if !token.Valid {
		return "", "", fmt.Errorf("invalid token")
	}
	if claims.UserID == "" {
		return "", "", fmt.Errorf("token missing user ID")
	}
	// Reject MFA-pending tokens from being used as full auth tokens
	if claims.MFAPending {
		return "", "", fmt.Errorf("mfa verification required")
	}
	return claims.UserID, claims.JTI, nil
}

// GenerateMFAToken creates a short-lived JWT for the MFA challenge step.
// This token cannot be used as a full auth token — it only proves password
// was correct and identifies which user needs to complete MFA.
func (s *Service) GenerateMFAToken(userID string) (string, error) {
	now := time.Now().UTC()
	claims := &Claims{
		UserID:     userID,
		MFAPending: true,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			Issuer:    "aegis",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// ValidateMFAToken parses a JWT and returns the user ID only if it is
// a valid MFA-pending token (not a full auth token).
func (s *Service) ValidateMFAToken(tokenStr string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return "", fmt.Errorf("parse mfa token: %w", err)
	}
	if !token.Valid {
		return "", fmt.Errorf("invalid mfa token")
	}
	if !claims.MFAPending {
		return "", fmt.Errorf("not an mfa token")
	}
	if claims.UserID == "" {
		return "", fmt.Errorf("mfa token missing user ID")
	}
	return claims.UserID, nil
}

// TokenTTL returns the configured token time-to-live.
func (s *Service) TokenTTL() time.Duration {
	return s.tokenTTL
}
