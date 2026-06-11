package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/pixelvide/aegis/server/internal/email/templates"
	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/models"
)

// ─── Profile Email Handlers ─────────────────────────────────────────────────

// handleListEmails returns all emails for the authenticated user.
func (s *Server) handleListEmails(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	emails, err := s.common.ListUserEmails(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list emails")
		return
	}
	if emails == nil {
		emails = []models.UserEmail{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"emails": emails,
	})
}

// handleAddEmail adds a new (unverified) email to the user's account.
func (s *Server) handleAddEmail(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}

	// Check if email is already in use
	existing, err := s.common.GetUserEmailByAddress(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "email already in use")
		return
	}

	email, err := s.common.AddUserEmail(r.Context(), user.ID, req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add email")
		return
	}

	// Auto-send verification email
	go s.sendVerificationEmail(user.ID, email.ID, email.Email)

	writeJSON(w, http.StatusCreated, map[string]any{
		"email": email,
	})
}

// handleRemoveEmail removes a non-primary email from the user's account.
func (s *Server) handleRemoveEmail(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	emailID := r.PathValue("id")
	if emailID == "" {
		writeError(w, http.StatusBadRequest, "email ID is required")
		return
	}

	if err := s.common.RemoveUserEmail(r.Context(), emailID, user.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleSetPrimaryEmail sets an email as the user's primary email.
func (s *Server) handleSetPrimaryEmail(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	emailID := r.PathValue("id")
	if emailID == "" {
		writeError(w, http.StatusBadRequest, "email ID is required")
		return
	}

	if err := s.common.SetPrimaryEmail(r.Context(), emailID, user.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Primary email updated",
	})
}

// handleSendEmailVerification sends a verification email.
func (s *Server) handleSendEmailVerification(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	emailID := r.PathValue("id")
	if emailID == "" {
		writeError(w, http.StatusBadRequest, "email ID is required")
		return
	}

	emailRecord, err := s.common.GetUserEmail(r.Context(), emailID, user.ID)
	if err != nil || emailRecord == nil {
		writeError(w, http.StatusNotFound, "email not found")
		return
	}

	if emailRecord.Verified {
		writeError(w, http.StatusConflict, "email is already verified")
		return
	}

	// Generate verification token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	expiresAt := time.Now().UTC().Add(24 * time.Hour)

	// Store the token in password_reset_tokens table (reusing for email verification)
	if err := s.common.CreatePasswordResetToken(r.Context(), user.ID, tokenHash, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Send verification email
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s&email_id=%s", s.config.BaseURL, token, emailID)
	subject, body := templates.VerifyEmail(verifyURL)

	if err := s.email.Send(emailRecord.Email, subject, body); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to send verification email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Verification email sent",
	})
}

// handleVerifyEmail verifies an email using a token.
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token   string `json:"token"`
		EmailID string `json:"email_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" || req.EmailID == "" {
		writeError(w, http.StatusBadRequest, "token and email_id are required")
		return
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.Token)))

	resetToken, err := s.common.GetPasswordResetToken(r.Context(), tokenHash)
	if err != nil || resetToken == nil {
		writeError(w, http.StatusBadRequest, "invalid or expired verification link")
		return
	}

	// Mark the token as used
	_ = s.common.MarkResetTokenUsed(r.Context(), resetToken.ID)

	// Mark the email as verified
	if err := s.common.MarkUserEmailVerified(r.Context(), req.EmailID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Email verified successfully",
	})
}
