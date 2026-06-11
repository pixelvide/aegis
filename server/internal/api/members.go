package api

import (
	"net/http"
	"net/mail"
	"strings"

	authpkg "github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

// handleListMembers returns all members of the current org.
func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	org := middleware.OrgFromContext(r.Context())
	if org == nil {
		writeError(w, http.StatusBadRequest, "org context required")
		return
	}

	members, err := s.common.ListOrgMembers(r.Context(), org.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	if members == nil {
		members = []store.MemberInfo{}
	}

	writeJSON(w, http.StatusOK, members)
}

type inviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// handleInviteMember adds a user to the current org. Creates the user if needed.
// Requires owner or admin role.
func (s *Server) handleInviteMember(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdminOrOwner(r.Context()) {
		writeError(w, http.StatusForbidden, "only owners and admins can invite members")
		return
	}

	org := middleware.OrgFromContext(r.Context())
	if org == nil {
		writeError(w, http.StatusBadRequest, "org context required")
		return
	}

	var req inviteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}

	role := req.Role
	if role == "" {
		role = "member"
	}
	if role != "owner" && role != "admin" && role != "member" && role != "viewer" {
		writeError(w, http.StatusBadRequest, "role must be owner, admin, member, or viewer")
		return
	}

	// Find or create the user
	user, err := s.common.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		// Create a placeholder user (no password — they'll register or get invited via email)
		user = &models.User{
			Email: req.Email,
			Name:  strings.Split(req.Email, "@")[0],
		}
		// Generate a random unusable password hash
		hash, _ := authpkg.HashPassword("__invited__placeholder__" + req.Email)
		user.PasswordHash = hash
		if err := s.common.CreateUser(r.Context(), user); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create user")
			return
		}
	}

	// Add to org
	if err := s.common.AddOrgMember(r.Context(), org.ID, user.ID, role); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add member")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user_id": user.ID,
		"email":   user.Email,
		"role":    role,
	})
}

// handleRemoveMember removes a user from the current org.
// Requires owner or admin role.
func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdminOrOwner(r.Context()) {
		writeError(w, http.StatusForbidden, "only owners and admins can remove members")
		return
	}

	org := middleware.OrgFromContext(r.Context())
	if org == nil {
		writeError(w, http.StatusBadRequest, "org context required")
		return
	}

	userID := pathParam(r, "userId")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID required")
		return
	}

	// Prevent removing yourself
	currentUser := middleware.UserFromContext(r.Context())
	if currentUser != nil && currentUser.ID == userID {
		writeError(w, http.StatusBadRequest, "cannot remove yourself from the organization")
		return
	}

	if err := s.common.RemoveOrgMember(r.Context(), org.ID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
