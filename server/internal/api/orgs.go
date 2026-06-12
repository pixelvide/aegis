package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

// slugPattern validates org slugs: lowercase alphanumeric + hyphens, 3-50 chars.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

type createOrgRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Plan string `json:"plan"`
}

// validPlans is the allowlist of accepted plan values.
var validPlans = map[string]bool{"free": true, "pro": true, "enterprise": true}

// handleCreateOrg creates a new organization and provisions its schema.
func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	var req createOrgRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("Name is required"))
		return
	}
	if len(req.Name) > 100 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("Name must be 100 characters or fewer"))
		return
	}

	slug := store.SanitizeSlug(req.Slug)
	if slug == "" {
		slug = store.SanitizeSlug(req.Name)
	}
	if !slugPattern.MatchString(slug) {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("Slug must be 3-50 lowercase alphanumeric characters or hyphens"))
		return
	}

	// Check reserved slugs early with a user-friendly error
	if store.IsReservedSlug(slug) {
		writeApiError(w, r, errTenantSlugReserved)
		return
	}

	// Check if slug is already taken
	existing, err := s.common.GetOrgBySlug(r.Context(), slug)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if existing != nil {
		writeApiError(w, r, errTenantSlugTaken)
		return
	}

	plan := req.Plan
	if plan == "" {
		plan = "free"
	}
	if !validPlans[plan] {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("Invalid plan: must be free, pro, or enterprise"))
		return
	}

	org := &models.Organization{
		Name: req.Name,
		Slug: slug,
		Plan: plan,
	}

	if err := s.common.CreateOrganization(r.Context(), org); err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	// Add the creator as owner
	user := middleware.UserFromContext(r.Context())
	if user != nil {
		s.common.AddOrgMember(r.Context(), org.ID, user.ID, "owner")
	}

	writeResult(w, r, http.StatusCreated, org)
}

// handleListOrgs returns the current user's organizations and app config.
func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeApiError(w, r, errAuthNotAuthenticated)
		return
	}

	orgs, err := s.common.GetUserOrgs(r.Context(), user.ID)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if orgs == nil {
		orgs = []models.Organization{}
	}
	writeResult(w, r, http.StatusOK, map[string]any{
		"orgs":        orgs,
		"base_domain": s.config.BaseDomain,
	})
}

// handleGetOrg returns an organization by slug.
func (s *Server) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	slug := pathParam(r, "slug")
	org, err := s.common.GetOrgBySlug(r.Context(), slug)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if org == nil {
		writeApiError(w, r, errTenantNotFound)
		return
	}
	writeResult(w, r, http.StatusOK, org)
}
