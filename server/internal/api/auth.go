package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"

	authpkg "github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/config"
	"github.com/pixelvide/aegis/server/internal/email/templates"
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
		writeApiError(w, r, errFeatureDisabled.WithMessage("Registration is currently disabled"))
		return
	}

	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	// Validate email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("Invalid email address"))
		return
	}

	// Validate password strength
	if err := validatePassword(req.Password); err != nil {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage(err.Error()))
		return
	}

	// Validate name
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("Name is required"))
		return
	}

	// Check if email already exists
	existing, err := s.common.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if existing != nil {
		writeApiError(w, r, errResourceAlreadyExists.WithMessage("Email already registered"))
		return
	}

	// Hash password
	hash, err := authpkg.HashPassword(req.Password)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	// Create user
	user := &models.User{
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hash,
	}
	if err := s.common.CreateUser(r.Context(), user); err != nil {
		slog.Error("failed to create user", "email", req.Email, "error", err)
		writeApiError(w, r, errServerInternal)
		return
	}
	slog.Info("user registered", "user_id", user.ID, "email", req.Email)

	// Create primary email entry + send verification
	primaryEmail, err := s.common.AddPrimaryUserEmail(r.Context(), user.ID, req.Email)
	if err != nil {
		// Non-fatal — user is created
		_ = err
	} else {
		// Send verification email (fire-and-forget)
		go s.sendVerificationEmail(user.ID, primaryEmail.ID, primaryEmail.Email)
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

	// Generate JWT + create session
	token, jti, err := s.auth.GenerateToken(user.ID)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	s.createSession(r, user.ID, jti)
	setAuthCookie(w, token, s.config)

	writeResultMessage(w, r, http.StatusCreated, map[string]any{
		"user": user,
	}, "Registration successful")
}

// handleLogin authenticates a user and returns a JWT cookie.
// If the user has MFA enabled, returns an MFA challenge instead.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("Email and password are required"))
		return
	}

	// Look up user
	user, err := s.common.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if user == nil {
		// Use same error for missing user and wrong password (prevent enumeration)
		writeApiError(w, r, errAuthInvalidCredentials)
		return
	}

	// Check password
	if err := authpkg.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeApiError(w, r, errAuthInvalidCredentials)
		return
	}

	// Check if MFA is enabled
	if user.MFAEnabled {
		// Get verified MFA devices for the user
		devices, err := s.common.GetVerifiedMFADevices(r.Context(), user.ID)
		if err != nil || len(devices) == 0 {
			// MFA enabled but no verified devices — shouldn't happen, allow login
			goto issueToken
		}

		// Password correct but MFA required — issue a short-lived MFA token
		mfaToken, err := s.auth.GenerateMFAToken(user.ID)
		if err != nil {
			writeApiError(w, r, errServerInternal)
			return
		}

		// Build methods list (mask email for privacy)
		methods := make([]map[string]string, len(devices))
		for i, d := range devices {
			methods[i] = map[string]string{
				"id":   d.ID,
				"type": d.Type,
				"name": d.Name,
			}
			if d.Type == "email" {
				methods[i]["name"] = maskEmail(d.Email)
			}
		}

		writeResult(w, r, http.StatusOK, map[string]any{
			"mfa_required": true,
			"mfa_token":    mfaToken,
			"mfa_methods":  methods,
		})
		return
	}

issueToken:
	// No MFA — generate full JWT + create session
	token, jti, err := s.auth.GenerateToken(user.ID)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	s.createSession(r, user.ID, jti)
	setAuthCookie(w, token, s.config)

	// Send login notification email
	ip := extractIP(r)
	browser, osName, deviceType := parseUserAgent(r.UserAgent())
	go s.sendLoginNotification(user.Email, user.Name, ip, browser, osName, deviceType)

	writeResult(w, r, http.StatusOK, map[string]any{
		"user": user,
	})
}

// handleLogout clears the auth cookie and revokes the session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Revoke current session if we have a JTI
	jti := middleware.JTIFromContext(r.Context())
	if jti != "" {
		_ = s.common.RevokeSessionByJTI(r.Context(), jti)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieName,
		Value:    "",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1, // delete immediately
		Domain:   cookieDomain(s.config),
	})
	writeMessage(w, r, http.StatusOK, "Logged out")
}

// handleMe returns the current authenticated user and their orgs.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeApiError(w, r, errAuthNotAuthenticated)
		return
	}

	orgs, err := s.common.GetUserOrgs(r.Context(), user.ID)
	if err != nil {
		orgs = []models.Organization{}
	}
	if orgs == nil {
		orgs = []models.Organization{}
	}

	writeResult(w, r, http.StatusOK, map[string]any{
		"user": user,
		"orgs": orgs,
	})
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func setAuthCookie(w http.ResponseWriter, token string, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CookieName,
		Value:    token,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		// When BaseDomain is set, scope cookie to parent domain for cross-subdomain sharing
		Domain: cookieDomain(cfg),
		// TODO(security): Enable Secure: true in production with HTTPS
		// TODO(security): Consider __Secure- cookie prefix when Secure is enabled
	})
}

// cookieDomain returns the domain to set on auth cookies.
// When BaseDomain is set, returns ".aegis.io" (leading dot) so the cookie
// is shared across all subdomains. When empty, returns "" (browser defaults
// to the exact host — standard dev mode behavior).
func cookieDomain(cfg *config.Config) string {
	if cfg.BaseDomain != "" {
		return "." + cfg.BaseDomain
	}
	return ""
}

// createSession creates a session row for the given user and JTI.
func (s *Server) createSession(r *http.Request, userID, jti string) {
	ua := r.UserAgent()
	browser, os, deviceType := parseUserAgent(ua)

	session := &models.UserSession{
		UserID:     userID,
		JTI:        jti,
		IPAddress:  extractIP(r),
		UserAgent:  ua,
		Browser:    browser,
		OS:         os,
		DeviceType: deviceType,
		ExpiresAt:  time.Now().UTC().Add(s.auth.TokenTTL()),
	}
	_ = s.common.CreateSession(r.Context(), session)
}

// extractIP gets the client IP from the request.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (reverse proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i]
	}
	return addr
}

// parseUserAgent extracts browser, OS, and device type from a User-Agent string.
func parseUserAgent(ua string) (browser, os, deviceType string) {
	// Browser detection
	switch {
	case strings.Contains(ua, "Edg/") || strings.Contains(ua, "Edge/"):
		browser = "Edge"
	case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
		browser = "Opera"
	case strings.Contains(ua, "Firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/") && !strings.Contains(ua, "Edg/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/") && !strings.Contains(ua, "Chrome"):
		browser = "Safari"
	default:
		browser = "Unknown"
	}

	// OS detection
	switch {
	case strings.Contains(ua, "Windows"):
		os = "Windows"
	case strings.Contains(ua, "Mac OS X") || strings.Contains(ua, "Macintosh"):
		os = "macOS"
	case strings.Contains(ua, "Android"):
		os = "Android"
	case strings.Contains(ua, "iPhone") || strings.Contains(ua, "iPad"):
		os = "iOS"
	case strings.Contains(ua, "Linux"):
		os = "Linux"
	case strings.Contains(ua, "CrOS"):
		os = "ChromeOS"
	default:
		os = "Unknown"
	}

	// Device type detection
	switch {
	case strings.Contains(ua, "Mobile") || strings.Contains(ua, "iPhone") || strings.Contains(ua, "Android"):
		if strings.Contains(ua, "Tablet") || strings.Contains(ua, "iPad") {
			deviceType = "tablet"
		} else {
			deviceType = "mobile"
		}
	case strings.Contains(ua, "iPad"):
		deviceType = "tablet"
	default:
		deviceType = "desktop"
	}

	return
}

// sendLoginNotification sends an email alert about a new login.
func (s *Server) sendLoginNotification(emailAddr, name, ip, browser, os, deviceType string) {
	loginTime := time.Now().UTC().Format("Jan 02, 2006 at 15:04 UTC")
	subject, body := templates.LoginAlert(name, ip, browser, os, deviceType, loginTime)
	if err := s.email.Send(emailAddr, subject, body); err != nil {
		slog.Error("failed to send login notification email", "email", emailAddr, "error", err)
	}
}

// maskEmail masks an email for privacy (e.g., "j***@example.com").
func maskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 1 {
		return email
	}
	return string(email[0]) + "***" + email[at:]
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

// sendVerificationEmail creates a verification token and emails it.
// Safe to call from a goroutine (uses background context).
func (s *Server) sendVerificationEmail(userID, emailID, emailAddr string) {
	ctx := context.Background()

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		slog.Error("failed to generate verification token", "user_id", userID, "email", emailAddr, "error", err)
		return
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	if err := s.common.CreatePasswordResetToken(ctx, userID, tokenHash, expiresAt); err != nil {
		slog.Error("failed to create verification token", "user_id", userID, "email", emailAddr, "error", err)
		return
	}

	verifyURL := fmt.Sprintf("%s/verify-email?token=%s&email_id=%s", s.config.BaseURL, token, emailID)
	subject, body := templates.VerifyEmail(verifyURL)

	if err := s.email.Send(emailAddr, subject, body); err != nil {
		slog.Error("failed to send verification email", "user_id", userID, "email", emailAddr, "error", err)
	}
}
