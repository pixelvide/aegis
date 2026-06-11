package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	authpkg "github.com/pixelvide/aegis/server/internal/auth"
	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/models"
)

// ─── MFA Device Management ─────────────────────────────────────────────────

// handleListMFADevices returns all MFA devices for the authenticated user.
func (s *Server) handleListMFADevices(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	devices, err := s.common.ListMFADevices(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	if devices == nil {
		devices = []models.MFADevice{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"devices":     devices,
		"mfa_enabled": user.MFAEnabled,
	})
}

// handleAddTOTPDevice creates a new TOTP device and returns the setup info.
func (s *Server) handleAddTOTPDevice(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		req.Name = "Authenticator"
	}

	// Generate TOTP key
	key, err := authpkg.GenerateTOTP(user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate TOTP key")
		return
	}

	// Create the device (unverified)
	device := &models.MFADevice{
		UserID: user.ID,
		Name:   req.Name,
		Type:   "totp",
		Secret: key.Secret(),
	}
	if err := s.common.CreateMFADevice(r.Context(), device); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create device")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"device":  device,
		"secret":  key.Secret(),
		"url":     key.URL(),
		"message": "Scan the QR code with your authenticator app, then verify with a code",
	})
}

// handleVerifyTOTPDevice verifies a TOTP device with a 6-digit code.
func (s *Server) handleVerifyTOTPDevice(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	deviceID := r.PathValue("id")
	if deviceID == "" {
		writeError(w, http.StatusBadRequest, "device ID is required")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	device, err := s.common.GetMFADevice(r.Context(), deviceID, user.ID)
	if err != nil || device == nil {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	if device.Type != "totp" {
		writeError(w, http.StatusBadRequest, "device is not a TOTP device")
		return
	}

	if device.Verified {
		writeError(w, http.StatusConflict, "device is already verified")
		return
	}

	// Validate the TOTP code against the device's secret
	if !authpkg.ValidateTOTP(device.Secret, req.Code) {
		writeError(w, http.StatusBadRequest, "invalid code — make sure your authenticator is synced")
		return
	}

	if err := s.common.VerifyMFADevice(r.Context(), device.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify device")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "TOTP device verified successfully",
	})
}

// handleAddEmailMFADevice creates an email OTP MFA device.
func (s *Server) handleAddEmailMFADevice(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req struct {
		EmailID string `json:"email_id"` // ID from user_emails table
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EmailID == "" {
		writeError(w, http.StatusBadRequest, "email_id is required")
		return
	}

	// Look up the email and verify it belongs to this user and is verified
	email, err := s.common.GetUserEmail(r.Context(), req.EmailID, user.ID)
	if err != nil || email == nil {
		writeError(w, http.StatusNotFound, "email not found")
		return
	}
	if !email.Verified {
		writeError(w, http.StatusBadRequest, "email must be verified before using for MFA")
		return
	}

	// Create the device (verified immediately since the email is already verified)
	device := &models.MFADevice{
		UserID: user.ID,
		Name:   email.Email,
		Type:   "email",
		Email:  email.Email,
	}
	if err := s.common.CreateMFADevice(r.Context(), device); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create device")
		return
	}

	// Auto-verify since the email itself is already verified
	if err := s.common.VerifyMFADevice(r.Context(), device.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify device")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"device":  device,
		"message": "Email MFA device added",
	})
}

// handleVerifyEmailMFADevice verifies an email MFA device with an OTP code.
func (s *Server) handleVerifyEmailMFADevice(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	deviceID := r.PathValue("id")
	if deviceID == "" {
		writeError(w, http.StatusBadRequest, "device ID is required")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Code = strings.TrimSpace(req.Code)

	device, err := s.common.GetMFADevice(r.Context(), deviceID, user.ID)
	if err != nil || device == nil {
		writeError(w, http.StatusNotFound, "device not found")
		return
	}

	if device.Type != "email" {
		writeError(w, http.StatusBadRequest, "device is not an email device")
		return
	}

	// Hash the code and validate
	codeHash := sha256Hash(req.Code)
	valid, err := s.common.ValidateEmailOTP(r.Context(), user.ID, device.ID, codeHash)
	if err != nil || !valid {
		writeError(w, http.StatusBadRequest, "invalid or expired code")
		return
	}

	if !device.Verified {
		if err := s.common.VerifyMFADevice(r.Context(), device.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify device")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Email MFA device verified",
	})
}

// handleRemoveMFADevice removes an MFA device (password required).
func (s *Server) handleRemoveMFADevice(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	deviceID := r.PathValue("id")
	if deviceID == "" {
		writeError(w, http.StatusBadRequest, "device ID is required")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	if err := authpkg.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}

	if err := s.common.RemoveMFADevice(r.Context(), deviceID, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove device")
		return
	}

	// Auto-disable MFA if no verified devices/emails remain
	mfaAutoDisabled := false
	if user.MFAEnabled {
		hasDevice, err1 := s.common.HasVerifiedMFADevice(r.Context(), user.ID)
		hasEmail, err2 := s.common.HasVerifiedEmail(r.Context(), user.ID)
		if err1 == nil && err2 == nil && !hasDevice && !hasEmail {
			_ = s.common.DisableMFA(r.Context(), user.ID)
			mfaAutoDisabled = true
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":           "MFA device removed",
		"mfa_auto_disabled": mfaAutoDisabled,
	})
}

// ─── MFA Toggle ─────────────────────────────────────────────────────────────

// handleEnableMFA enables MFA for the user (requires verified device or verified email).
func (s *Server) handleEnableMFA(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if user.MFAEnabled {
		writeError(w, http.StatusConflict, "MFA is already enabled")
		return
	}

	// Check prerequisite: at least one verified TOTP device is required
	hasDevice, err := s.common.HasVerifiedMFADevice(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if !hasDevice {
		writeError(w, http.StatusBadRequest, "add and verify at least one TOTP authenticator device first")
		return
	}

	// Generate 8 recovery codes
	plainCodes, err := authpkg.GenerateRecoveryCodes(8)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate recovery codes")
		return
	}

	hashes := make([]string, len(plainCodes))
	for i, code := range plainCodes {
		h, err := authpkg.HashPassword(code)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		hashes[i] = h
	}

	if err := s.common.CreateRecoveryCodes(r.Context(), user.ID, hashes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store recovery codes")
		return
	}

	if err := s.common.EnableMFA(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enable MFA")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":        "MFA has been enabled",
		"recovery_codes": plainCodes,
	})
}

// handleDisableMFA disables MFA (password required).
func (s *Server) handleDisableMFA(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if !user.MFAEnabled {
		writeError(w, http.StatusBadRequest, "MFA is not enabled")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	if err := authpkg.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}

	if err := s.common.DisableMFA(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disable MFA")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "MFA has been disabled",
	})
}

// ─── Recovery Codes ─────────────────────────────────────────────────────────

// handleRecoveryCodesCount returns the number of unused recovery codes.
func (s *Server) handleRecoveryCodesCount(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	count, err := s.common.CountUnusedRecoveryCodes(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"remaining": count,
		"total":     8,
	})
}

// handleRegenerateRecoveryCodes regenerates recovery codes (password required).
func (s *Server) handleRegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if !user.MFAEnabled {
		writeError(w, http.StatusBadRequest, "MFA must be enabled to regenerate recovery codes")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "password is required")
		return
	}

	if err := authpkg.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}

	plainCodes, err := authpkg.GenerateRecoveryCodes(8)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	hashes := make([]string, len(plainCodes))
	for i, code := range plainCodes {
		h, err := authpkg.HashPassword(code)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		hashes[i] = h
	}

	if err := s.common.CreateRecoveryCodes(r.Context(), user.ID, hashes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store recovery codes")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recovery_codes": plainCodes,
		"message":        "Recovery codes regenerated. Old codes are now invalid.",
	})
}

// ─── MFA Login Flow ─────────────────────────────────────────────────────────

// handleMFAValidate completes login when MFA is required.
func (s *Server) handleMFAValidate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MFAToken   string `json:"mfa_token"`
		DeviceID   string `json:"device_id"`
		Code       string `json:"code"`
		IsRecovery bool   `json:"is_recovery"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.MFAToken = strings.TrimSpace(req.MFAToken)
	req.Code = strings.TrimSpace(req.Code)
	if req.MFAToken == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, "mfa_token and code are required")
		return
	}

	// Validate the MFA pending token
	userID, err := s.auth.ValidateMFAToken(req.MFAToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired MFA token")
		return
	}

	user, err := s.common.GetUser(r.Context(), userID)
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	if req.IsRecovery {
		// Try each stored recovery code (bcrypt comparison)
		matched := s.tryRecoveryCode(r, user.ID, req.Code)
		if !matched {
			writeError(w, http.StatusUnauthorized, "invalid recovery code")
			return
		}
	} else if req.DeviceID != "" {
		// Validate against a specific device
		device, err := s.common.GetMFADevice(r.Context(), req.DeviceID, user.ID)
		if err != nil || device == nil || !device.Verified {
			writeError(w, http.StatusUnauthorized, "invalid device")
			return
		}

		switch device.Type {
		case "totp":
			if !authpkg.ValidateTOTP(device.Secret, req.Code) {
				writeError(w, http.StatusUnauthorized, "invalid TOTP code")
				return
			}
		case "email":
			codeHash := sha256Hash(req.Code)
			valid, err := s.common.ValidateEmailOTP(r.Context(), user.ID, device.ID, codeHash)
			if err != nil || !valid {
				writeError(w, http.StatusUnauthorized, "invalid or expired email code")
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "unsupported device type")
			return
		}

		// Update last used
		_ = s.common.UpdateMFADeviceLastUsed(r.Context(), device.ID)
	} else {
		writeError(w, http.StatusBadRequest, "device_id is required")
		return
	}

	// MFA verified — issue full auth token + session
	token, jti, err := s.auth.GenerateToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	s.createSession(r, user.ID, jti)
	setAuthCookie(w, token)

	// Send login notification email
	ip := extractIP(r)
	browser, osName, deviceType := parseUserAgent(r.UserAgent())
	go s.sendLoginNotification(user.Email, user.Name, ip, browser, osName, deviceType)

	response := map[string]any{
		"user": user,
	}
	if req.IsRecovery {
		remaining, _ := s.common.CountUnusedRecoveryCodes(r.Context(), user.ID)
		response["recovery_codes_remaining"] = remaining
	}

	writeJSON(w, http.StatusOK, response)
}

// handleSendEmailOTP sends an OTP code to an email MFA device during login.
func (s *Server) handleSendEmailOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MFAToken string `json:"mfa_token"`
		DeviceID string `json:"device_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.MFAToken == "" || req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, "mfa_token and device_id are required")
		return
	}

	userID, err := s.auth.ValidateMFAToken(req.MFAToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired MFA token")
		return
	}

	device, err := s.common.GetMFADevice(r.Context(), req.DeviceID, userID)
	if err != nil || device == nil || !device.Verified || device.Type != "email" {
		writeError(w, http.StatusNotFound, "email MFA device not found")
		return
	}

	// Generate 6-digit OTP
	code, err := generateOTP(6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate code")
		return
	}

	codeHash := sha256Hash(code)
	expiresAt := time.Now().UTC().Add(5 * time.Minute)

	if err := s.common.CreateEmailOTP(r.Context(), userID, device.ID, codeHash, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store OTP")
		return
	}

	// Send the OTP via email
	subject := "Your Aegis verification code"
	body := fmt.Sprintf("Your verification code is: %s\n\nThis code expires in 5 minutes.", code)
	if err := s.email.Send(device.Email, subject, body); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to send email")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Verification code sent to " + maskEmail(device.Email),
	})
}

// ─── MFA Helpers ────────────────────────────────────────────────────────────

// tryRecoveryCode attempts to match a plaintext recovery code against stored hashes.
func (s *Server) tryRecoveryCode(r *http.Request, userID, code string) bool {
	rows, err := s.common.DB().QueryContext(r.Context(),
		"SELECT id, code_hash FROM common.mfa_recovery_codes WHERE user_id = $1 AND used_at IS NULL",
		userID)
	if err != nil {
		return false
	}
	defer rows.Close()

	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return false
		}
		if authpkg.CheckPassword(hash, code) == nil {
			// Mark as used
			_, _ = s.common.DB().ExecContext(r.Context(),
				"UPDATE common.mfa_recovery_codes SET used_at = NOW() WHERE id = $1", id)
			return true
		}
	}
	return false
}

// sha256Hash returns the hex-encoded SHA-256 hash of a string.
func sha256Hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// generateOTP creates a random numeric OTP of the given length.
func generateOTP(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	otp := make([]byte, length)
	for i := range otp {
		otp[i] = '0' + b[i]%10
	}
	return string(otp), nil
}
