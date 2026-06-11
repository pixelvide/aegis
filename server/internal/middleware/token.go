package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/pixelvide/aegis/server/internal/config"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

const (
	// AgentTokenKey is the context key for the authenticated API token.
	AgentTokenKey contextKey = "aegis_agent_token"

	// tokenPrefixLen is the length of the prefix stored for lookup: "aegis_" + 8 hex = 14 chars.
	tokenPrefixLen = 14
)

// AgentTokenFromContext extracts the authenticated API token from the request context.
func AgentTokenFromContext(ctx context.Context) *models.APIToken {
	t, _ := ctx.Value(AgentTokenKey).(*models.APIToken)
	return t
}

// TokenAuth returns middleware that authenticates via Bearer token.
//
// Flow:
//  1. Extract "Bearer aegis_xxx" from Authorization header
//  2. Resolve org from subdomain (Host header) or X-Org-Slug fallback
//  3. Load tenant store for that org
//  4. Look up token by prefix in org's api_tokens table
//  5. Verify full token against stored bcrypt hash
//  6. Check: not revoked, not expired
//  7. Inject org + store + token into context
//  8. Update last_used asynchronously
func TokenAuth(common *store.CommonStore, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error":"authorization header required"}`, http.StatusUnauthorized)
				return
			}

			const bearerPrefix = "Bearer "
			if !strings.HasPrefix(authHeader, bearerPrefix) {
				http.Error(w, `{"error":"invalid authorization format, use: Bearer <token>"}`, http.StatusUnauthorized)
				return
			}

			rawToken := strings.TrimPrefix(authHeader, bearerPrefix)
			rawToken = strings.TrimSpace(rawToken)

			// Validate token format: aegis_ + 32 hex chars
			if !isValidTokenFormat(rawToken) {
				http.Error(w, `{"error":"invalid token format"}`, http.StatusUnauthorized)
				return
			}

			// 2. Resolve org from subdomain or header
			var org *models.Organization
			var err error

			// Try subdomain first (production)
			if cfg.BaseDomain != "" {
				host := r.Host
				// Strip port if present
				if idx := strings.LastIndex(host, ":"); idx != -1 {
					host = host[:idx]
				}
				subdomain := extractSubdomain(host, cfg.BaseDomain)
				if subdomain != "" {
					org, err = common.GetOrgBySlug(r.Context(), subdomain)
				}
			}

			// Fallback to X-Org-Slug or X-Org-ID headers (dev mode)
			if org == nil && err == nil {
				if orgID := r.Header.Get("X-Org-ID"); orgID != "" {
					org, err = common.GetOrganization(r.Context(), orgID)
				} else if slug := r.Header.Get("X-Org-Slug"); slug != "" {
					org, err = common.GetOrgBySlug(r.Context(), slug)
				}
			}

			// Try custom domain lookup
			if org == nil && err == nil && cfg.BaseDomain != "" {
				host := r.Host
				if idx := strings.LastIndex(host, ":"); idx != -1 {
					host = host[:idx]
				}
				if !strings.HasSuffix(host, cfg.BaseDomain) {
					org, err = common.GetOrgByDomain(r.Context(), host)
				}
			}

			if err != nil {
				http.Error(w, `{"error":"internal error resolving organization"}`, http.StatusInternalServerError)
				return
			}
			if org == nil {
				http.Error(w, `{"error":"organization not found"}`, http.StatusUnauthorized)
				return
			}

			// 3. Create tenant store
			tenantStore := store.NewTenantStore(common.DB(), org.SchemaName())

			// 4. Look up token by prefix
			prefix := rawToken[:tokenPrefixLen]
			token, hash, err := tenantStore.GetAPITokenByPrefix(r.Context(), prefix)
			if err != nil {
				http.Error(w, `{"error":"internal error checking token"}`, http.StatusInternalServerError)
				return
			}
			if token == nil {
				http.Error(w, `{"error":"invalid or revoked token"}`, http.StatusUnauthorized)
				return
			}

			// 5. Check expiration
			if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now().UTC()) {
				http.Error(w, `{"error":"token has expired"}`, http.StatusUnauthorized)
				return
			}

			// 6. Verify token against bcrypt hash
			if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(rawToken)); err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			// 7. Inject context values
			ctx := context.WithValue(r.Context(), OrgContextKey, org)
			ctx = context.WithValue(ctx, TenantStoreKey, tenantStore)
			ctx = context.WithValue(ctx, AgentTokenKey, token)

			// 8. Update last_used async (non-blocking)
			go func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				tenantStore.UpdateTokenLastUsed(bgCtx, token.ID)
			}()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractSubdomain extracts the subdomain from a host given a base domain.
// e.g., "acme.aegis.io" with baseDomain "aegis.io" returns "acme".
// Returns "" if no valid subdomain is found.
func extractSubdomain(host, baseDomain string) string {
	suffix := "." + baseDomain
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	sub := strings.TrimSuffix(host, suffix)
	// Ensure it's a simple subdomain (no dots = no nested subdomains)
	if strings.Contains(sub, ".") || sub == "" {
		return ""
	}
	return sub
}

// isValidTokenFormat checks that a token matches "aegis_" followed by 32 hex chars.
func isValidTokenFormat(token string) bool {
	const prefix = "aegis_"
	if !strings.HasPrefix(token, prefix) {
		return false
	}
	hex := token[len(prefix):]
	if len(hex) != 32 {
		return false
	}
	for _, c := range hex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
