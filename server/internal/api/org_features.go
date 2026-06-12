package api

import (
	"log/slog"
	"net/http"
	"regexp"

	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/store"
)

// orgFeatureFlagNamePattern validates flag names: lowercase letters, digits, underscores.
var orgFeatureFlagNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

// handleListOrgFeatures returns all org-level feature flags.
// Any authenticated org member can read.
func (s *Server) handleListOrgFeatures(w http.ResponseWriter, r *http.Request) {
	ts := tenantStore(r)
	flags, err := ts.ListOrgFeatureFlags(r.Context())
	if err != nil {
		slog.Error("list org feature flags", "error", err)
		writeApiError(w, r, errServerInternal)
		return
	}
	if flags == nil {
		flags = []store.OrgFeatureFlag{}
	}
	writeResult(w, r, http.StatusOK, flags)
}

// updateOrgFeatureRequest is the body for PATCH /api/v1/org-features/{flag}.
type updateOrgFeatureRequest struct {
	Enabled bool `json:"enabled"`
}

// handleUpdateOrgFeature toggles the enabled state of an org feature flag.
// Only the org owner can toggle. The flag must exist and be provisioned.
func (s *Server) handleUpdateOrgFeature(w http.ResponseWriter, r *http.Request) {
	flagName := pathParam(r, "flag")

	// Validate flag name format
	if !orgFeatureFlagNamePattern.MatchString(flagName) {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("invalid feature flag name"))
		return
	}

	// RBAC: owner only
	role := middleware.RoleFromContext(r.Context())
	if role != "owner" {
		writeApiError(w, r, errPermissionDenied.WithMessage("only the organization owner can toggle feature flags"))
		return
	}

	var req updateOrgFeatureRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	ts := tenantStore(r)
	if err := ts.SetOrgFeatureEnabled(r.Context(), flagName, req.Enabled); err != nil {
		// Check if the error is "not provisioned" vs an actual DB error
		writeApiError(w, r, errFeatureNotProvisioned.WithMessage(err.Error()))
		return
	}

	writeMessage(w, r, http.StatusOK, "feature flag updated")
}
