package middleware

import (
	"context"
	"net/http"

	"github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

const (
	// CookieName is the name of the auth cookie.
	CookieName = "aegis_token"

	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "aegis_user"
)

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(UserContextKey).(*models.User)
	return user
}

// Auth returns middleware that validates the JWT cookie and injects the user into context.
func Auth(authSvc *auth.Service, common *store.CommonStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(CookieName)
			if err != nil || cookie.Value == "" {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}

			userID, err := authSvc.ValidateToken(cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			user, err := common.GetUser(r.Context(), userID)
			if err != nil || user == nil {
				http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
