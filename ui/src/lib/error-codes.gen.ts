// ─── Auto-generated from errors.yaml ────────────────────────────────────────
// DO NOT EDIT MANUALLY. Run `go generate ./internal/api/...` to regenerate.

// ErrorType represents the category of an error.
export type ErrorType =
  | "auth_error"
  | "token_error"
  | "tenant_error"
  | "resource_error"
  | "validation_error"
  | "permission_error"
  | "rate_limit_error"
  | "server_error"

// ErrorCode represents a specific error code.
export type ErrorCode =
  // auth_error
  | "not_authenticated"
  | "invalid_credentials"
  | "email_not_verified"
  | "mfa_required"
  | "mfa_invalid_code"
  | "session_revoked"
  | "base_domain_only"
  // token_error
  | "invalid"
  | "expired"
  | "scope_mismatch"
  | "invalid_format"
  // tenant_error
  | "not_found"
  | "not_member"
  | "slug_taken"
  | "slug_reserved"
  | "header_conflict"
  // resource_error
  | "conflict"
  | "already_exists"
  // validation_error
  | "invalid_request"
  | "invalid_body"
  | "field_required"
  | "field_invalid"
  // permission_error
  | "denied"
  | "feature_disabled"
  | "mfa_required_by_org"
  // rate_limit_error
  | "exceeded"
  // server_error
  | "internal"
  | "unavailable"

// ErrorEntry describes a single error code's metadata.
export interface ErrorEntry {
  type: ErrorType
  code: ErrorCode
  ref: string
  status: number
  message: string
}

// ERROR_CODES maps "type.code" to its metadata for programmatic lookup.
export const ERROR_CODES: Record<string, ErrorEntry> = {
  // auth_error
  "auth_error.not_authenticated": { type: "auth_error", code: "not_authenticated", ref: "E10001", status: 401, message: "Authentication required" },
  "auth_error.invalid_credentials": { type: "auth_error", code: "invalid_credentials", ref: "E10002", status: 401, message: "Invalid email or password" },
  "auth_error.email_not_verified": { type: "auth_error", code: "email_not_verified", ref: "E10003", status: 403, message: "Please verify your email address before continuing" },
  "auth_error.mfa_required": { type: "auth_error", code: "mfa_required", ref: "E10004", status: 403, message: "MFA verification required" },
  "auth_error.mfa_invalid_code": { type: "auth_error", code: "mfa_invalid_code", ref: "E10005", status: 401, message: "Invalid or expired verification code" },
  "auth_error.session_revoked": { type: "auth_error", code: "session_revoked", ref: "E10006", status: 401, message: "Session has been revoked" },
  "auth_error.base_domain_only": { type: "auth_error", code: "base_domain_only", ref: "E10007", status: 403, message: "Authentication is only available on the base domain" },

  // token_error
  "token_error.invalid": { type: "token_error", code: "invalid", ref: "E20001", status: 401, message: "Invalid or revoked token" },
  "token_error.expired": { type: "token_error", code: "expired", ref: "E20002", status: 401, message: "Token has expired" },
  "token_error.scope_mismatch": { type: "token_error", code: "scope_mismatch", ref: "E20003", status: 403, message: "Token does not have access to this resource" },
  "token_error.invalid_format": { type: "token_error", code: "invalid_format", ref: "E20004", status: 401, message: "Invalid authorization format" },

  // tenant_error
  "tenant_error.not_found": { type: "tenant_error", code: "not_found", ref: "E30001", status: 404, message: "Organization not found" },
  "tenant_error.not_member": { type: "tenant_error", code: "not_member", ref: "E30002", status: 403, message: "You are not a member of this organization" },
  "tenant_error.slug_taken": { type: "tenant_error", code: "slug_taken", ref: "E30003", status: 409, message: "Organization slug is already taken" },
  "tenant_error.slug_reserved": { type: "tenant_error", code: "slug_reserved", ref: "E30004", status: 409, message: "Organization slug is reserved" },
  "tenant_error.header_conflict": { type: "tenant_error", code: "header_conflict", ref: "E30005", status: 400, message: "Org header conflicts with subdomain" },

  // resource_error
  "resource_error.not_found": { type: "resource_error", code: "not_found", ref: "E40001", status: 404, message: "Resource not found" },
  "resource_error.conflict": { type: "resource_error", code: "conflict", ref: "E40002", status: 409, message: "Resource conflict" },
  "resource_error.already_exists": { type: "resource_error", code: "already_exists", ref: "E40003", status: 409, message: "Resource already exists" },

  // validation_error
  "validation_error.invalid_request": { type: "validation_error", code: "invalid_request", ref: "E50001", status: 400, message: "Invalid request" },
  "validation_error.invalid_body": { type: "validation_error", code: "invalid_body", ref: "E50002", status: 400, message: "Invalid request body" },
  "validation_error.field_required": { type: "validation_error", code: "field_required", ref: "E50003", status: 400, message: "Required field missing" },
  "validation_error.field_invalid": { type: "validation_error", code: "field_invalid", ref: "E50004", status: 400, message: "Invalid field value" },

  // permission_error
  "permission_error.denied": { type: "permission_error", code: "denied", ref: "E60001", status: 403, message: "Insufficient permissions" },
  "permission_error.feature_disabled": { type: "permission_error", code: "feature_disabled", ref: "E60002", status: 403, message: "Feature is not enabled" },
  "permission_error.mfa_required_by_org": { type: "permission_error", code: "mfa_required_by_org", ref: "E60003", status: 403, message: "This organization requires MFA to be enabled" },

  // rate_limit_error
  "rate_limit_error.exceeded": { type: "rate_limit_error", code: "exceeded", ref: "E70001", status: 429, message: "Too many requests" },

  // server_error
  "server_error.internal": { type: "server_error", code: "internal", ref: "E90001", status: 500, message: "Internal server error" },
  "server_error.unavailable": { type: "server_error", code: "unavailable", ref: "E90002", status: 503, message: "Service temporarily unavailable" },
} as const

// isErrorCode checks if a string is a known error code.
export function isErrorCode(code: string): code is ErrorCode {
  return Object.values(ERROR_CODES).some(e => e.code === code)
}
