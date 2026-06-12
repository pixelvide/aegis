package middleware

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"github.com/pixelvide/aegis/server/internal/requestid"
)

// RequestID is a middleware that generates a unique request ID for every request.
//
// It checks for an incoming X-Request-ID header from a reverse proxy or load
// balancer. If present, it reuses that ID. Otherwise, it generates a new one
// using a time-based prefix and cryptographic random suffix (format: req_<hex>).
//
// The request ID is:
//   - Stored in the request context (accessible via requestid.FromContext)
//   - Set as the X-Request-ID response header
//
// This middleware must be the outermost in the chain so that even auth failures
// and 404s have a request ID for log correlation.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}

		// Set response header before any downstream writes
		w.Header().Set("X-Request-ID", reqID)

		// Inject into context for downstream handlers and logger
		ctx := requestid.WithRequestID(r.Context(), reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID produces a unique request ID with format: req_<timestamp_hex><random_hex>.
// Total: "req_" + 12 hex chars (48-bit ms timestamp) + 16 hex chars (64-bit random) = 32 chars.
// Time-ordered for log sorting, cryptographically random suffix for uniqueness.
func generateRequestID() string {
	// 48-bit millisecond timestamp (good until year 10889)
	ms := uint64(time.Now().UnixMilli())
	ts := fmt.Sprintf("%012x", ms)

	// 64-bit cryptographic random suffix
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Fallback: use nanosecond timestamp as entropy (should never happen)
		ns := uint64(time.Now().UnixNano())
		for i := range buf {
			buf[i] = byte(ns >> (i * 8))
		}
	}
	rnd := fmt.Sprintf("%016x", buf)

	return "req_" + ts + rnd
}
