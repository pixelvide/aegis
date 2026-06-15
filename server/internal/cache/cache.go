// Package cache provides a Valkey/Redis-backed cache layer.
//
// Used for session revocation checks (avoiding PostgreSQL on every
// authenticated request) and MFA brute-force protection (attempt
// counting and token allowlisting).
//
// Valkey is a required dependency — the server will not start without it.
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// SessionTTL is how long session status is cached before re-checking the DB.
const SessionTTL = 5 * time.Minute

// MFASessionTTL matches the MFA token JWT expiry (5 minutes).
const MFASessionTTL = 5 * time.Minute

// MFAMaxAttempts is the maximum number of failed MFA code attempts per token.
const MFAMaxAttempts = 5

// OTPCooldown is the minimum interval between email OTP sends per device.
const OTPCooldown = 30 * time.Second

// Client wraps a Valkey/Redis connection.
type Client struct {
	rdb *redis.Client
}

// New creates a new cache client connected to the given Valkey address.
// addr format: "host:port" (e.g., "valkey:6379").
func New(addr string) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		DB:           0,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("valkey ping failed: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

// Close shuts down the cache connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// ─── Session Cache ──────────────────────────────────────────────────────────

// sessionKey returns the cache key for a session's active status.
func sessionKey(jti string) string {
	return "session:active:" + jti
}

// revokedKey returns the cache key for a revoked session.
func revokedKey(jti string) string {
	return "session:revoked:" + jti
}

// MarkSessionActive caches that a session is active (not revoked).
func (c *Client) MarkSessionActive(ctx context.Context, jti string) {
	if err := c.rdb.Set(ctx, sessionKey(jti), "1", SessionTTL).Err(); err != nil {
		slog.Error("cache: failed to mark session active", "error", err, "jti", jti)
	}
}

// MarkSessionRevoked caches that a session has been revoked.
// This is set with a longer TTL since revocations are permanent.
func (c *Client) MarkSessionRevoked(ctx context.Context, jti string) {
	// Delete active marker and set revoked marker
	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, sessionKey(jti))
	pipe.Set(ctx, revokedKey(jti), "1", 24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		slog.Error("cache: failed to mark session revoked", "error", err, "jti", jti)
	}
}

// IsSessionRevoked checks the cache for session revocation status.
// Returns:
//
//	revoked=true  → session is known to be revoked (skip DB)
//	revoked=false, cached=true  → session is known to be active (skip DB)
//	revoked=false, cached=false → cache miss, must check DB
func (c *Client) IsSessionRevoked(ctx context.Context, jti string) (revoked bool, cached bool) {
	// Check revoked marker first
	if val, err := c.rdb.Get(ctx, revokedKey(jti)).Result(); err == nil && val == "1" {
		return true, true
	}

	// Check active marker
	if val, err := c.rdb.Get(ctx, sessionKey(jti)).Result(); err == nil && val == "1" {
		return false, true
	}

	return false, false // cache miss
}

// InvalidateUserSessions removes all cached session data for a user.
// Called when revoking all sessions.
func (c *Client) InvalidateUserSessions(ctx context.Context, jtis []string) {
	if len(jtis) == 0 {
		return
	}
	keys := make([]string, 0, len(jtis)*2)
	for _, jti := range jtis {
		keys = append(keys, sessionKey(jti), revokedKey(jti))
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		slog.Error("cache: failed to invalidate user sessions", "error", err, "count", len(jtis))
	}
}

// ─── MFA Brute-Force Protection ─────────────────────────────────────────────

// mfaSessionKey returns the Valkey key for an MFA token allowlist entry.
func mfaSessionKey(jti string) string {
	return "mfa:session:" + jti
}

// mfaAttemptsKey returns the Valkey key for an MFA token's failure counter.
func mfaAttemptsKey(jti string) string {
	return "mfa:attempts:" + jti
}

// otpCooldownKey returns the Valkey key for email OTP send cooldown.
func otpCooldownKey(jti, deviceID string) string {
	return "mfa:otp_cd:" + jti + ":" + deviceID
}

// RegisterMFASession stores an MFA token JTI in the allowlist.
// Called when login returns mfa_required, before sending the MFA challenge.
// The allowlist entry stores the user ID and expires with the MFA token (5 min).
func (c *Client) RegisterMFASession(ctx context.Context, jti, userID string) error {
	if err := c.rdb.Set(ctx, mfaSessionKey(jti), userID, MFASessionTTL).Err(); err != nil {
		return fmt.Errorf("cache: register MFA session: %w", err)
	}
	return nil
}

// ValidateMFASession checks if an MFA token JTI is in the allowlist.
// Returns the stored user ID if the session is valid.
// Returns an error if the session is not found (token exhausted, expired, or never registered).
func (c *Client) ValidateMFASession(ctx context.Context, jti string) (string, error) {
	userID, err := c.rdb.Get(ctx, mfaSessionKey(jti)).Result()
	if err != nil {
		return "", fmt.Errorf("cache: MFA session not found or expired: %w", err)
	}
	return userID, nil
}

// IncrementMFAAttempts increments the failure counter for an MFA token.
// Returns the new attempt count after incrementing.
// If the count reaches MFAMaxAttempts, the MFA session is automatically
// invalidated (removed from the allowlist), making the token permanently dead.
func (c *Client) IncrementMFAAttempts(ctx context.Context, jti string) (int64, error) {
	key := mfaAttemptsKey(jti)

	// Increment atomically
	count, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("cache: increment MFA attempts: %w", err)
	}

	// Set TTL on first attempt (INCR creates the key if it doesn't exist)
	if count == 1 {
		c.rdb.Expire(ctx, key, MFASessionTTL)
	}

	// Auto-invalidate the MFA session when max attempts reached
	if count >= int64(MFAMaxAttempts) {
		c.rdb.Del(ctx, mfaSessionKey(jti))
		slog.Warn("MFA token exhausted — max attempts reached",
			"jti", jti, "attempts", count)
	}

	return count, nil
}

// InvalidateMFASession removes an MFA token from the allowlist.
// Called after successful MFA validation (one-time use) or when max attempts reached.
func (c *Client) InvalidateMFASession(ctx context.Context, jti string) error {
	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, mfaSessionKey(jti))
	pipe.Del(ctx, mfaAttemptsKey(jti))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache: invalidate MFA session: %w", err)
	}
	return nil
}

// CheckOTPCooldown checks if an email OTP was recently sent for this
// MFA token + device combination. Returns true if still in cooldown.
func (c *Client) CheckOTPCooldown(ctx context.Context, jti, deviceID string) (bool, error) {
	exists, err := c.rdb.Exists(ctx, otpCooldownKey(jti, deviceID)).Result()
	if err != nil {
		return false, fmt.Errorf("cache: check OTP cooldown: %w", err)
	}
	return exists > 0, nil
}

// SetOTPCooldown marks that an email OTP was just sent for this
// MFA token + device combination. Expires after OTPCooldown (30s).
func (c *Client) SetOTPCooldown(ctx context.Context, jti, deviceID string) error {
	if err := c.rdb.Set(ctx, otpCooldownKey(jti, deviceID), "1", OTPCooldown).Err(); err != nil {
		return fmt.Errorf("cache: set OTP cooldown: %w", err)
	}
	return nil
}
