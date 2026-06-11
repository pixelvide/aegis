package middleware

import (
	"context"
	"net/http"

	"github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/cache"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

const (
	// CookieName is the name of the auth cookie.
	CookieName = "aegis_token"

	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "aegis_user"

	// JTIContextKey is the context key for the session JTI.
	JTIContextKey contextKey = "aegis_jti"
)

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(UserContextKey).(*models.User)
	return user
}

// JTIFromContext extracts the session JTI from the request context.
func JTIFromContext(ctx context.Context) string {
	jti, _ := ctx.Value(JTIContextKey).(string)
	return jti
}

// Auth returns middleware that validates the JWT cookie, checks session
// revocation (cache first, then DB), and injects the user + JTI into context.
// cache may be nil — falls back to DB-only checks.
func Auth(authSvc *auth.Service, common *store.CommonStore, sessionCache *cache.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}

			userID, jti, err := authSvc.ValidateToken(cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Check if the session has been revoked (only for JTI-bearing tokens)
			if jti != "" {
				// Try cache first (Valkey)
				if revoked, cached := sessionCache.IsSessionRevoked(r.Context(), jti); cached {
					if revoked {
						http.Error(w, `{"error":"session has been revoked"}`, http.StatusUnauthorized)
						return
					}
					// Cache says active — skip DB check
				} else {
					// Cache miss — check DB
					revoked, err := common.IsSessionRevoked(r.Context(), jti)
					if err != nil || revoked {
						if revoked {
							sessionCache.MarkSessionRevoked(r.Context(), jti)
						}
						http.Error(w, `{"error":"session has been revoked"}`, http.StatusUnauthorized)
						return
					}
					// Populate cache with active status
					sessionCache.MarkSessionActive(r.Context(), jti)
				}

				// Touch session last_active_at (fire-and-forget, non-blocking)
				go func() {
					_ = common.TouchSession(context.Background(), jti)
				}()
			}

			user, err := common.GetUser(r.Context(), userID)
			if err != nil || user == nil {
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			ctx = context.WithValue(ctx, JTIContextKey, jti)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
