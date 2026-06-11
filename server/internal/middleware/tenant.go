// Package middleware provides HTTP middleware for the Aegis server.
package middleware

import (
	"context"
	"net/http"

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
//  1. X-Org-ID header (UUID)
//  2. X-Org-Slug header (slug)
//
// If an authenticated user is in context, verifies org membership.
func TenantResolver(common *store.CommonStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var org *models.Organization
			var err error

			// Try X-Org-ID first, then X-Org-Slug
			if orgID := r.Header.Get("X-Org-ID"); orgID != "" {
				org, err = common.GetOrganization(r.Context(), orgID)
			} else if slug := r.Header.Get("X-Org-Slug"); slug != "" {
				org, err = common.GetOrgBySlug(r.Context(), slug)
			}

			if err != nil {
				http.Error(w, `{"error":"internal error resolving org"}`, http.StatusInternalServerError)
				return
			}
			if org == nil {
				http.Error(w, `{"error":"organization not found, set X-Org-ID or X-Org-Slug header"}`, http.StatusBadRequest)
				return
			}

			// Verify the authenticated user has access to this org and get their role
			var memberRole string
			user := UserFromContext(r.Context())
			if user != nil {
				role, err := common.GetMemberRole(r.Context(), org.ID, user.ID)
				if err != nil {
					http.Error(w, `{"error":"internal error checking membership"}`, http.StatusInternalServerError)
					return
				}
				if role == "" {
					http.Error(w, `{"error":"you are not a member of this organization"}`, http.StatusForbidden)
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
