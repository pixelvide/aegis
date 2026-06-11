package api

import (
	"net/http"
	"net/mail"
	"strings"
	"unicode"

	authpkg "github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// handleRegister creates a new user, a default org, and returns a JWT cookie.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Check if signup is enabled
	if !s.common.IsFeatureEnabled(r.Context(), "signup") {
		writeError(w, http.StatusForbidden, "registration is currently disabled")
		return
	}

	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}

	// Validate password strength
	if err := validatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate name
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Check if email already exists
	existing, err := s.common.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "email already registered")
		return
	}

	// Hash password
	hash, err := authpkg.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Create user
	user := &models.User{
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hash,
	}
	if err := s.common.CreateUser(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	// Create default org from user's name
	orgSlug := store.SanitizeSlug(req.Name)
	if len(orgSlug) < 3 {
		orgSlug = orgSlug + "-org"
	}
	org := &models.Organization{
		Name: req.Name + "'s Org",
		Slug: orgSlug,
		Plan: "free",
	}
	if err := s.common.CreateOrganization(r.Context(), org); err != nil {
		// Non-fatal — user is created, just no default org
		_ = err
	} else {
		// Make user the owner of the new org
		s.common.AddOrgMember(r.Context(), org.ID, user.ID, "owner")
	}

	// Generate JWT
	token, err := s.auth.GenerateToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	setAuthCookie(w, token)

	writeJSON(w, http.StatusCreated, map[string]any{
		"user": user,
		"message": "registration successful",
	})
}

// handleLogin authenticates a user and returns a JWT cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Look up user
	user, err := s.common.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		// Use same error for missing user and wrong password (prevent enumeration)
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Check password
	if err := authpkg.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Generate JWT
	token, err := s.auth.GenerateToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	setAuthCookie(w, token)

	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
	})
}

// handleLogout clears the auth cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieName,
		Value:    "",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1, // delete immediately
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// handleMe returns the current authenticated user and their orgs.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	orgs, err := s.common.GetUserOrgs(r.Context(), user.ID)
	if err != nil {
		orgs = []models.Organization{}
	}
	if orgs == nil {
		orgs = []models.Organization{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": user,
		"orgs": orgs,
	})
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieName,
		Value:    token,
		HttpOnly: true,
		// Secure: true, // enable in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   86400, // 24 hours
	})
}

// validatePassword enforces minimum security requirements.
func validatePassword(pw string) error {
	if len(pw) < 8 {
		return &passwordError{"password must be at least 8 characters"}
	}
	var hasUpper, hasLower, hasDigit bool
	for _, c := range pw {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		return &passwordError{"password must contain uppercase, lowercase, and a digit"}
	}
	return nil
}

type passwordError struct{ msg string }

func (e *passwordError) Error() string { return e.msg }
