package api

import "net/http"

// ─── Pre-defined API Errors ─────────────────────────────────────────────────
// These will eventually be auto-generated from errors.yaml via `go generate`.
// For now they are manually defined to match the error code registry.

// Auth errors
var (
	errAuthNotAuthenticated = ApiError{
		Type: "auth_error", Code: "not_authenticated", Ref: "E10001",
		Status: http.StatusUnauthorized, Message: "Authentication required",
	}
	errAuthInvalidCredentials = ApiError{
		Type: "auth_error", Code: "invalid_credentials", Ref: "E10002",
		Status: http.StatusUnauthorized, Message: "Invalid email or password",
	}
	errAuthEmailNotVerified = ApiError{
		Type: "auth_error", Code: "email_not_verified", Ref: "E10003",
		Status: http.StatusForbidden, Message: "Please verify your email address before continuing",
	}
	errAuthMFARequired = ApiError{
		Type: "auth_error", Code: "mfa_required", Ref: "E10004",
		Status: http.StatusForbidden, Message: "MFA verification required",
	}
	errAuthMFAInvalidCode = ApiError{
		Type: "auth_error", Code: "mfa_invalid_code", Ref: "E10005",
		Status: http.StatusUnauthorized, Message: "Invalid or expired verification code",
	}
	errAuthSessionRevoked = ApiError{
		Type: "auth_error", Code: "session_revoked", Ref: "E10006",
		Status: http.StatusUnauthorized, Message: "Session has been revoked",
	}
	errAuthBaseDomainOnly = ApiError{
		Type: "auth_error", Code: "base_domain_only", Ref: "E10007",
		Status: http.StatusForbidden, Message: "Authentication is only available on the base domain",
	}
)

// Token errors
var (
	errTokenInvalid = ApiError{
		Type: "token_error", Code: "invalid", Ref: "E20001",
		Status: http.StatusUnauthorized, Message: "Invalid or revoked token",
	}
	errTokenExpired = ApiError{
		Type: "token_error", Code: "expired", Ref: "E20002",
		Status: http.StatusUnauthorized, Message: "Token has expired",
	}
	errTokenScopeMismatch = ApiError{
		Type: "token_error", Code: "scope_mismatch", Ref: "E20003",
		Status: http.StatusForbidden, Message: "Token does not have access to this resource",
	}
	errTokenInvalidFormat = ApiError{
		Type: "token_error", Code: "invalid_format", Ref: "E20004",
		Status: http.StatusUnauthorized, Message: "Invalid authorization format",
	}
)

// Tenant errors
var (
	errTenantNotFound = ApiError{
		Type: "tenant_error", Code: "not_found", Ref: "E30001",
		Status: http.StatusNotFound, Message: "Organization not found",
	}
	errTenantNotMember = ApiError{
		Type: "tenant_error", Code: "not_member", Ref: "E30002",
		Status: http.StatusForbidden, Message: "You are not a member of this organization",
	}
	errTenantSlugTaken = ApiError{
		Type: "tenant_error", Code: "slug_taken", Ref: "E30003",
		Status: http.StatusConflict, Message: "Organization slug is already taken",
	}
	errTenantSlugReserved = ApiError{
		Type: "tenant_error", Code: "slug_reserved", Ref: "E30004",
		Status: http.StatusConflict, Message: "Organization slug is reserved",
	}
	errTenantHeaderConflict = ApiError{
		Type: "tenant_error", Code: "header_conflict", Ref: "E30005",
		Status: http.StatusBadRequest, Message: "Org header conflicts with subdomain",
	}
)

// Resource errors
var (
	errResourceNotFound = ApiError{
		Type: "resource_error", Code: "not_found", Ref: "E40001",
		Status: http.StatusNotFound, Message: "Resource not found",
	}
	errResourceConflict = ApiError{
		Type: "resource_error", Code: "conflict", Ref: "E40002",
		Status: http.StatusConflict, Message: "Resource conflict",
	}
	errResourceAlreadyExists = ApiError{
		Type: "resource_error", Code: "already_exists", Ref: "E40003",
		Status: http.StatusConflict, Message: "Resource already exists",
	}
)

// Validation errors
var (
	errValidationInvalidRequest = ApiError{
		Type: "validation_error", Code: "invalid_request", Ref: "E50001",
		Status: http.StatusBadRequest, Message: "Invalid request",
	}
	errValidationInvalidBody = ApiError{
		Type: "validation_error", Code: "invalid_body", Ref: "E50002",
		Status: http.StatusBadRequest, Message: "Invalid request body",
	}
	errValidationFieldRequired = ApiError{
		Type: "validation_error", Code: "field_required", Ref: "E50003",
		Status: http.StatusBadRequest, Message: "Required field missing",
	}
	errValidationFieldInvalid = ApiError{
		Type: "validation_error", Code: "field_invalid", Ref: "E50004",
		Status: http.StatusBadRequest, Message: "Invalid field value",
	}
)

// Permission errors
var (
	errPermissionDenied = ApiError{
		Type: "permission_error", Code: "denied", Ref: "E60001",
		Status: http.StatusForbidden, Message: "Insufficient permissions",
	}
	errFeatureDisabled = ApiError{
		Type: "permission_error", Code: "feature_disabled", Ref: "E60002",
		Status: http.StatusForbidden, Message: "Feature is not enabled",
	}
	errMFARequiredByOrg = ApiError{
		Type: "permission_error", Code: "mfa_required_by_org", Ref: "E60003",
		Status: http.StatusForbidden, Message: "This organization requires MFA to be enabled",
	}
)

// Rate limit errors
var (
	errRateLimitExceeded = ApiError{
		Type: "rate_limit_error", Code: "exceeded", Ref: "E70001",
		Status: http.StatusTooManyRequests, Message: "Too many requests",
	}
)

// Server errors
var (
	errServerInternal = ApiError{
		Type: "server_error", Code: "internal", Ref: "E90001",
		Status: http.StatusInternalServerError, Message: "Internal server error",
	}
	errServerUnavailable = ApiError{
		Type: "server_error", Code: "unavailable", Ref: "E90002",
		Status: http.StatusServiceUnavailable, Message: "Service temporarily unavailable",
	}
)
