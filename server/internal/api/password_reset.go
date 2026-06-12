package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"

	authpkg "github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/email/templates"
	"github.com/pixelvide/aegis/server/internal/middleware"
)

// ─── Password Reset ─────────────────────────────────────────────────────────

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleForgotPassword generates a reset token and sends it via email.
// Always returns 200 regardless of whether the email exists (prevent enumeration).
func (s *Server) handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("invalid email address"))
		return
	}

	// Always return same response (prevent email enumeration)
	successMsg := map[string]string{"message": "If that email is registered, a password reset link has been sent"}

	user, err := s.common.GetUserByEmail(r.Context(), req.Email)
	if err != nil || user == nil {
		writeResult(w, r, http.StatusOK, successMsg)
		return
	}

	// Generate a cryptographically secure reset token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	rawToken := hex.EncodeToString(tokenBytes)

	// Store SHA-256 hash of the token (not the token itself)
	tokenHash := hashToken(rawToken)
	expiresAt := time.Now().UTC().Add(1 * time.Hour)

	if err := s.common.CreatePasswordResetToken(r.Context(), user.ID, tokenHash, expiresAt); err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	// Send reset email — BaseURL (from APP_BASE_URL) always points to the base domain
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.config.BaseURL, rawToken)

	subject, htmlBody := templates.PasswordReset(resetURL)

	if err := s.email.Send(user.Email, subject, htmlBody); err != nil {
		// Log error but don't reveal it to the user (email enumeration)
		slog.Error("failed to send password reset email", "email", user.Email, "error", err)
	}

	writeResult(w, r, http.StatusOK, successMsg)
}

// handleResetPassword validates the reset token and updates the password.
func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	req.Token = strings.TrimSpace(req.Token)
	if req.Token == "" || req.Password == "" {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("token and password are required"))
		return
	}

	// Validate password strength
	if err := validatePassword(req.Password); err != nil {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage(err.Error()))
		return
	}

	// Look up token by hash
	tokenHash := hashToken(req.Token)
	resetToken, err := s.common.GetPasswordResetToken(r.Context(), tokenHash)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if resetToken == nil {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("invalid or expired reset token"))
		return
	}

	// Hash the new password
	newHash, err := authpkg.HashPassword(req.Password)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	// Update password
	if err := s.common.UpdateUserPassword(r.Context(), resetToken.UserID, newHash); err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	// Mark token as used
	s.common.MarkResetTokenUsed(r.Context(), resetToken.ID)

	// Invalidate all other reset tokens for this user
	s.common.InvalidateResetTokens(r.Context(), resetToken.UserID)

	// Send notification email
	if resetUser, err := s.common.GetUser(r.Context(), resetToken.UserID); err == nil && resetUser != nil {
		go s.sendPasswordChangedNotification(resetUser.Email, resetUser.Name)
	}

	writeMessage(w, r, http.StatusOK, "password has been reset successfully")
}

// handleChangePassword allows an authenticated user to change their password.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeApiError(w, r, errAuthNotAuthenticated)
		return
	}

	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("current_password and new_password are required"))
		return
	}

	// Verify current password
	if err := authpkg.CheckPassword(user.PasswordHash, req.CurrentPassword); err != nil {
		writeApiError(w, r, errAuthNotAuthenticated)
		return
	}

	// Validate new password strength
	if err := validatePassword(req.NewPassword); err != nil {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage(err.Error()))
		return
	}

	// Hash and update
	newHash, err := authpkg.HashPassword(req.NewPassword)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	if err := s.common.UpdateUserPassword(r.Context(), user.ID, newHash); err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	// Send notification email
	go s.sendPasswordChangedNotification(user.Email, user.Name)

	writeMessage(w, r, http.StatusOK, "password changed successfully")
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// hashToken creates a SHA-256 hex digest of a raw token string.
// Used for password reset tokens — we store the hash, not the plaintext.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// sendPasswordChangedNotification sends a security alert email when a password is changed.
func (s *Server) sendPasswordChangedNotification(emailAddr, name string) {
	subject, body := templates.PasswordChanged(name)
	if err := s.email.Send(emailAddr, subject, body); err != nil {
		slog.Error("failed to send password changed notification", "email", emailAddr, "error", err)
	}
}
