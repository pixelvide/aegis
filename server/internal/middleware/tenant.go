// Package middleware provides HTTP middleware for the Aegis server.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/pixelvide/aegis/server/internal/config"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

type contextKey string

const (
	// OrgContextKey is the context key for the current organization.
	OrgContextKey contextKey = "aegis_org"
	// TenantStoreKey is the context key for the tenant-scoped store.
	TenantStoreKey contextKey = "aegis_tenant_store"
	// MemberRoleKey is the context key for the user's role in the current org.
	MemberRoleKey contextKey = "aegis_member_role"
)

// OrgFromContext extracts the current organization from the request context.
func OrgFromContext(ctx context.Context) *models.Organization {
	org, _ := ctx.Value(OrgContextKey).(*models.Organization)
	return org
}

// TenantStoreFromContext extracts the tenant-scoped store from the request context.
func TenantStoreFromContext(ctx context.Context) store.Store {
	s, _ := ctx.Value(TenantStoreKey).(store.Store)
	return s
}

// RoleFromContext extracts the user's role in the current org.
func RoleFromContext(ctx context.Context) string {
	r, _ := ctx.Value(MemberRoleKey).(string)
	return r
}

// IsAdminOrOwner checks if the user's role is owner or admin.
func IsAdminOrOwner(ctx context.Context) bool {
	r := RoleFromContext(ctx)
	return r == "owner" || r == "admin"
}

// TenantResolver resolves the current org from the request and injects
// a schema-scoped Store into the context.
//
// Resolution order:
//  1. Subdomain (if AEGIS_BASE_DOMAIN is set): acme.aegis.io → slug "acme"
//  2. Custom domain: security.acme.com → lookup in organizations.custom_domain
//  3. X-Org-ID header (UUID)
//  4. X-Org-Slug header (slug)
//
// When AEGIS_BASE_DOMAIN is set and a subdomain is present, headers are ignored
// to prevent confused-deputy attacks (user on acme.aegis.io sending X-Org-Slug: other-org).
//
// If an authenticated user is in context, verifies org membership.
func TenantResolver(common *store.CommonStore, cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var org *models.Organization
			var err error
			var resolvedFromDomain bool

			// 1. Try subdomain resolution (production mode)
			if cfg.BaseDomain != "" {
				host := r.Host
				if idx := strings.LastIndex(host, ":"); idx != -1 {
					host = host[:idx]
				}
				subdomain := extractSubdomain(host, cfg.BaseDomain)
				if subdomain != "" {
					org, err = common.GetOrgBySlug(r.Context(), subdomain)
					resolvedFromDomain = true
				}
			}

			// 2. Try custom domain lookup
			if org == nil && err == nil && cfg.BaseDomain != "" {
				host := r.Host
				if idx := strings.LastIndex(host, ":"); idx != -1 {
					host = host[:idx]
				}
				if !strings.HasSuffix(host, cfg.BaseDomain) {
					org, err = common.GetOrgByDomain(r.Context(), host)
					if org != nil {
						resolvedFromDomain = true
					}
				}
			}

			// 3. If domain resolved the org, reject conflicting headers
			if resolvedFromDomain && org != nil {
				if headerID := r.Header.Get("X-Org-ID"); headerID != "" && headerID != org.ID {
					writeMiddlewareError(w, r, errTenantHeaderConflictID)
					return
				}
				if headerSlug := r.Header.Get("X-Org-Slug"); headerSlug != "" && headerSlug != org.Slug {
					writeMiddlewareError(w, r, errTenantHeaderConflictSlug)
					return
				}
			}

			// 4. Fallback to headers (dev mode, no subdomain)
			if org == nil && err == nil {
				if orgID := r.Header.Get("X-Org-ID"); orgID != "" {
					org, err = common.GetOrganization(r.Context(), orgID)
				} else if slug := r.Header.Get("X-Org-Slug"); slug != "" {
					org, err = common.GetOrgBySlug(r.Context(), slug)
				}
			}

			if err != nil {
				writeMiddlewareError(w, r, errServerInternal)
				return
			}
			if org == nil {
				writeMiddlewareError(w, r, errTenantNotFound)
				return
			}

			// Verify the authenticated user has access to this org and get their role
			var memberRole string
			user := UserFromContext(r.Context())
			if user != nil {
				role, err := common.GetMemberRole(r.Context(), org.ID, user.ID)
				if err != nil {
					writeMiddlewareError(w, r, errServerInternal)
					return
				}
				if role == "" {
					writeMiddlewareError(w, r, errTenantNotMember)
					return
				}
				memberRole = role
			}

			// Create a tenant-scoped store for this org
			tenantStore := store.NewTenantStore(common.DB(), org.SchemaName())

			// Inject org + store + role into context
			ctx := context.WithValue(r.Context(), OrgContextKey, org)
			ctx = context.WithValue(ctx, TenantStoreKey, tenantStore)
			ctx = context.WithValue(ctx, MemberRoleKey, memberRole)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

