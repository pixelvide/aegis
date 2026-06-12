package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/pixelvide/aegis/server/internal/requestid"
)

// MiddlewareError represents a structured error response for middleware.
// This mirrors api.ApiError but lives in the middleware package to avoid
// circular imports (middleware cannot import api).
type MiddlewareError struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Ref     string `json:"ref"`
	Status  int    `json:"-"`
	Message string `json:"message"`
}

// middlewareErrorEnvelope is the Cloudflare-style error response envelope.
type middlewareErrorEnvelope struct {
	Success   bool              `json:"success"`
	RequestID string            `json:"request_id,omitempty"`
	Errors    []MiddlewareError `json:"errors"`
}

// writeMiddlewareError writes a structured error response from middleware.
// Produces the same JSON shape as api.writeApiError:
//
//	{"success": false, "request_id": "req_...", "errors": [{...}]}
func writeMiddlewareError(w http.ResponseWriter, r *http.Request, errs ...MiddlewareError) {
	if len(errs) == 0 {
		return
	}

	env := middlewareErrorEnvelope{
		Success:   false,
		RequestID: requestid.FromContext(r.Context()),
		Errors:    errs,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(errs[0].Status)
	if err := json.NewEncoder(w).Encode(env); err != nil {
		slog.Error("middleware json encode error", "error", err)
	}
}

// ─── Pre-defined Middleware Errors ──────────────────────────────────────────
// These mirror the error codes from the registry (errors.yaml) but are defined
// as Go vars here to avoid importing api. They MUST stay in sync with errors.yaml.

var (
	// Auth errors
	errAuthRequired = MiddlewareError{
		Type: "auth_error", Code: "not_authenticated", Ref: "E10001",
		Status: http.StatusUnauthorized, Message: "Authentication required",
	}
	errAuthInvalidToken = MiddlewareError{
		Type: "auth_error", Code: "invalid_token", Ref: "E10001",
		Status: http.StatusUnauthorized, Message: "Invalid or expired token",
	}
	errAuthSessionRevoked = MiddlewareError{
		Type: "auth_error", Code: "session_revoked", Ref: "E10006",
		Status: http.StatusUnauthorized, Message: "Session has been revoked",
	}
	errAuthUserNotFound = MiddlewareError{
		Type: "auth_error", Code: "user_not_found", Ref: "E10001",
		Status: http.StatusUnauthorized, Message: "User not found",
	}
	errAuthEmailNotVerified = MiddlewareError{
		Type: "auth_error", Code: "email_not_verified", Ref: "E10003",
		Status: http.StatusForbidden, Message: "Please verify your email address before continuing",
	}

	// Token errors
	errTokenAuthRequired = MiddlewareError{
		Type: "token_error", Code: "auth_required", Ref: "E20004",
		Status: http.StatusUnauthorized, Message: "Authorization header required",
	}
	errTokenInvalidFormat = MiddlewareError{
		Type: "token_error", Code: "invalid_format", Ref: "E20004",
		Status: http.StatusUnauthorized, Message: "Invalid authorization format, use: Bearer <token>",
	}
	errTokenInvalid = MiddlewareError{
		Type: "token_error", Code: "invalid", Ref: "E20001",
		Status: http.StatusUnauthorized, Message: "Invalid or revoked token",
	}
	errTokenExpired = MiddlewareError{
		Type: "token_error", Code: "expired", Ref: "E20002",
		Status: http.StatusUnauthorized, Message: "Token has expired",
	}
	errTokenInvalidVerify = MiddlewareError{
		Type: "token_error", Code: "invalid", Ref: "E20001",
		Status: http.StatusUnauthorized, Message: "Invalid token",
	}

	// Tenant errors
	errTenantNotFound = MiddlewareError{
		Type: "tenant_error", Code: "not_found", Ref: "E30001",
		Status: http.StatusBadRequest, Message: "Organization not found, set X-Org-ID or X-Org-Slug header",
	}
	errTenantNotMember = MiddlewareError{
		Type: "tenant_error", Code: "not_member", Ref: "E30002",
		Status: http.StatusForbidden, Message: "You are not a member of this organization",
	}
	errTenantHeaderConflictID = MiddlewareError{
		Type: "tenant_error", Code: "header_conflict", Ref: "E30005",
		Status: http.StatusBadRequest, Message: "X-Org-ID header conflicts with subdomain",
	}
	errTenantHeaderConflictSlug = MiddlewareError{
		Type: "tenant_error", Code: "header_conflict", Ref: "E30005",
		Status: http.StatusBadRequest, Message: "X-Org-Slug header conflicts with subdomain",
	}
	errTenantOrgNotFoundAuth = MiddlewareError{
		Type: "tenant_error", Code: "not_found", Ref: "E30001",
		Status: http.StatusUnauthorized, Message: "Organization not found",
	}

	// Server errors
	errServerInternal = MiddlewareError{
		Type: "server_error", Code: "internal", Ref: "E90001",
		Status: http.StatusInternalServerError, Message: "Internal server error",
	}

	// MFA required by org
	errMFARequiredByOrg = MiddlewareError{
		Type: "permission_error", Code: "mfa_required_by_org", Ref: "E60003",
		Status: http.StatusForbidden, Message: "This organization requires MFA to be enabled",
	}
)
