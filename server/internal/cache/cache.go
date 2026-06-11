// Package cache provides a Valkey/Redis-backed cache layer.
//
// Used primarily for session revocation checks to avoid
// hitting PostgreSQL on every authenticated request.
// Falls back gracefully to the database on cache miss or error.
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
	if c == nil || c.rdb == nil {
		return nil
	}
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
	if c == nil {
		return
	}
	if err := c.rdb.Set(ctx, sessionKey(jti), "1", SessionTTL).Err(); err != nil {
		slog.Error("cache: failed to mark session active", "error", err, "jti", jti)
	}
}

// MarkSessionRevoked caches that a session has been revoked.
// This is set with a longer TTL since revocations are permanent.
func (c *Client) MarkSessionRevoked(ctx context.Context, jti string) {
	if c == nil {
		return
	}
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
	if c == nil {
		return false, false // no cache → always check DB
	}

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
	if c == nil || len(jtis) == 0 {
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
