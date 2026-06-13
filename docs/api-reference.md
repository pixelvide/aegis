# API Reference

Base URL: `http://lvh.me:8080/api/v1` (development) or `https://your-domain.com/api/v1` (production)

## Authentication

### User Auth (Cookie)
All authenticated endpoints require the `aegis_token` cookie (set by login/register).  
Org-scoped endpoints require requests to be made on an org subdomain (e.g., `acme.aegis.io`) or a custom domain.

### Agent Auth (Bearer Token)
Agent Ingest API endpoints use `Authorization: Bearer aegis_xxx` tokens instead of cookies.  
Org context is resolved from the request's subdomain or custom domain.

---

## Response Format

All API responses use a **Cloudflare-style envelope**. Every response includes `success` (boolean) and `request_id` (trace ID for debugging).

### Success — Single Resource

```json
{
  "success": true,
  "request_id": "req_019eb9d04166f69d...",
  "result": {
    "id": "uuid",
    "name": "Example"
  }
}
```

### Success — With Message

Action confirmations (register, login, logout, password reset, etc.) include an optional `message`:

```json
{
  "success": true,
  "request_id": "req_...",
  "result": { "user": {...} },
  "message": "Registration successful"
}
```

### Success — Message Only

For operations with no data payload (logout, password changed, etc.):

```json
{
  "success": true,
  "request_id": "req_...",
  "message": "Logged out"
}
```

### Success — Paginated List

```json
{
  "success": true,
  "request_id": "req_...",
  "result": [{...}, {...}],
  "result_info": {
    "page": 1,
    "per_page": 25,
    "total": 142,
    "total_pages": 6,
    "has_next": true,
    "has_prev": false
  }
}
```

### Error

Errors return `success: false` with a structured `errors` array:

```json
{
  "success": false,
  "request_id": "req_019eb9d0a040...",
  "errors": [
    {
      "type": "auth_error",
      "code": "invalid_credentials",
      "ref": "E10002",
      "message": "Invalid email or password"
    }
  ]
}
```

Each error has:

| Field | Description |
|---|---|
| `type` | Error category: `auth_error`, `token_error`, `tenant_error`, `resource_error`, `validation_error`, `permission_error`, `rate_limit_error`, `server_error` |
| `code` | Machine-readable error code (stable contract for programmatic handling) |
| `ref` | Numeric reference ID (e.g., `E10002`) for support tickets and log correlation |
| `message` | Human-readable description (may change — do not match on this) |
| `details` | *(optional)* Field-level validation errors: `[{"field": "email", "message": "invalid format"}]` |

### Empty (204 No Content)

DELETE operations return `204 No Content` with no body.

### Request ID

Every response includes:
- `request_id` field in the JSON body
- `X-Request-ID` response header

Use this for debugging: search server logs with `grep req_019eb9d0a040 server.log` to find the full request context.

---

## Auth

### POST `/auth/register`

Create a new account. Gated by the `signup` feature flag.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecureP@ss1",
  "name": "John Doe"
}
```

**Response (201):**
```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "John Doe",
    "avatar_url": "",
    "created_at": "2026-01-01T00:00:00Z"
  },
  "message": "registration successful"
}
```

**Errors:**
- `400` — Invalid email, weak password, missing name
- `403` — Registration disabled
- `409` — Email already registered

**Password Requirements:**
- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit

---

### POST `/auth/login`

Sign in with email and password. Sets `aegis_token` HttpOnly cookie.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecureP@ss1"
}
```

**Response (200):**
```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "John Doe",
    "avatar_url": "",
    "created_at": "2026-01-01T00:00:00Z"
  }
}
```

**Errors:**
- `401` — Invalid email or password (intentionally same message to prevent enumeration)

---

### POST `/auth/logout`

Clear the auth cookie.

**Response (200):**
```json
{ "message": "logged out" }
```

---

### GET `/auth/me` 🔒

Returns the current user and their organizations.

**Response (200):**
```json
{
  "user": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "John Doe",
    "avatar_url": "",
    "mfa_enabled": false,
    "created_at": "2026-01-01T00:00:00Z"
  },
  "orgs": [
    {
      "id": "uuid",
      "name": "My Org",
      "slug": "my-org",
      "plan": "free",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

---

## Password Reset

### POST `/auth/forgot-password`

Request a password reset email. Always returns 200 regardless of whether the email exists (prevents enumeration).

**Request:**
```json
{ "email": "user@example.com" }
```

**Response (200):**
```json
{ "message": "If that email is registered, a password reset link has been sent" }
```

---

### POST `/auth/reset-password`

Reset password using the token from the email link. Token is single-use and expires after 1 hour.

**Request:**
```json
{
  "token": "hex-encoded-token-from-email",
  "password": "NewSecureP@ss1"
}
```

**Response (200):**
```json
{ "message": "password has been reset successfully" }
```

**Errors:**
- `400` — Invalid/expired token, weak password

---

### POST `/auth/change-password` 🔒

Change password for the authenticated user. Requires current password verification.

**Request:**
```json
{
  "current_password": "OldP@ss1",
  "new_password": "NewSecureP@ss1"
}
```

**Response (200):**
```json
{ "message": "password changed successfully" }
```

**Errors:**
- `401` — Current password incorrect
- `400` — Weak new password

---

## Multi-Factor Authentication (MFA)

### Login with MFA

When a user with MFA enabled calls `POST /auth/login`, instead of setting the auth cookie, the server returns an MFA challenge:

**Response (200) — MFA required:**
```json
{
  "mfa_required": true,
  "mfa_token": "short-lived-jwt"
}
```

The `mfa_token` is valid for 5 minutes and must be submitted to `/auth/mfa/validate` with a TOTP code.

---

### POST `/auth/mfa/validate`

Complete login when MFA is required. Accepts a TOTP code, email OTP, or recovery code.

**Request (TOTP or email OTP):**
```json
{
  "mfa_token": "jwt-from-login-response",
  "device_id": "device-uuid",
  "code": "123456",
  "is_recovery": false
}
```

**Request (Recovery code):**
```json
{
  "mfa_token": "jwt-from-login-response",
  "code": "a1b2c3d4",
  "is_recovery": true
}
```

**Response (200):** Sets auth cookie + returns user. If a recovery code was used, includes remaining count:
```json
{
  "user": { "..." },
  "recovery_codes_remaining": 7
}
```

**Errors:**
- `401` — Invalid/expired MFA token, invalid code

---

### POST `/auth/mfa/send-email-otp`

Send an OTP code to an email MFA device during login (before full auth).

**Request:**
```json
{
  "mfa_token": "jwt-from-login-response",
  "device_id": "email-device-uuid"
}
```

**Response (200):**
```json
{ "message": "Verification code sent to j***@example.com" }
```

---

### POST `/auth/verify-email`

Verify an email address using the token from the email link. Token is single-use and expires after 24 hours.

**Request:**
```json
{
  "token": "hex-encoded-token-from-email",
  "email_id": "user-email-uuid"
}
```

**Response (200):**
```json
{ "message": "Email verified successfully" }
```

**Errors:**
- `400` — Invalid/expired verification link

---

## Profile — Emails 🔒

### GET `/profile/emails`

List all email addresses for the authenticated user.

**Response (200):**
```json
{
  "emails": [
    {
      "id": "uuid",
      "email": "user@example.com",
      "is_primary": true,
      "verified": true,
      "verified_at": "2026-01-01T00:00:00Z",
      "created_at": "2026-01-01T00:00:00Z"
    }
  ]
}
```

---

### POST `/profile/emails`

Add a new (unverified) email address.

**Request:**
```json
{ "email": "new@example.com" }
```

**Response (201):** Email object.

**Errors:**
- `400` — Invalid email
- `409` — Email already in use

---

### DELETE `/profile/emails/{id}`

Remove a non-primary email address.

**Response (204):** No content.

**Errors:**
- `400` — Cannot remove primary email

---

### POST `/profile/emails/{id}/set-primary`

Set a verified email as the user's primary email.

**Response (200):**
```json
{ "message": "Primary email updated" }
```

**Errors:**
- `400` — Email not verified, or not found

---

### POST `/profile/emails/{id}/send-verification`

Send a verification email link to the specified email address.

**Response (200):**
```json
{ "message": "Verification email sent" }
```

**Errors:**
- `409` — Email is already verified

---

## Profile — MFA 🔒

### GET `/profile/mfa/devices`

List all MFA devices for the authenticated user.

**Response (200):**
```json
{
  "devices": [
    { "id": "uuid", "name": "Work Phone", "type": "totp", "verified": true, "last_used_at": "..." }
  ],
  "mfa_enabled": true
}
```

---

### POST `/profile/mfa/devices/totp`

Create a new TOTP device (unverified). Returns setup info.

**Request:**
```json
{ "name": "Work Phone" }
```

**Response (201):**
```json
{
  "device": { "id": "uuid", "name": "Work Phone", "type": "totp", "verified": false },
  "secret": "BASE32SECRET",
  "url": "otpauth://totp/Aegis:user@example.com?secret=BASE32SECRET&issuer=Aegis"
}
```

---

### POST `/profile/mfa/devices/totp/{id}/verify`

Verify a TOTP device with a 6-digit code.

**Request:**
```json
{ "code": "123456" }
```

**Response (200):**
```json
{ "message": "TOTP device verified successfully" }
```

---

### POST `/profile/mfa/devices/email`

Create an email OTP MFA device from a verified email address.

**Request:**
```json
{ "email_id": "user-email-uuid" }
```

**Response (201):** Device object.

**Errors:**
- `400` — Email must be verified first

---

### DELETE `/profile/mfa/devices/{id}`

Remove an MFA device (password required).

**Request:**
```json
{ "password": "CurrentP@ss1" }
```

**Response (200):**
```json
{ "message": "MFA device removed" }
```

---

### POST `/profile/mfa/enable`

Enable MFA. Requires at least one verified device or verified email. Generates 8 recovery codes.

**Response (200):**
```json
{
  "message": "MFA has been enabled",
  "recovery_codes": ["abcd1234", "efgh5678", "..."]
}
```

> ⚠️ Save the recovery codes immediately — they cannot be retrieved again.

**Errors:**
- `400` — No verified device or email
- `409` — MFA already enabled

---

### POST `/profile/mfa/disable`

Disable MFA. Requires password confirmation. Deletes recovery codes.

**Request:**
```json
{ "password": "CurrentP@ss1" }
```

**Response (200):**
```json
{ "message": "MFA has been disabled" }
```

---

### GET `/profile/mfa/recovery-codes`

Get the count of unused recovery codes.

**Response (200):**
```json
{ "remaining": 6, "total": 8 }
```

---

### POST `/profile/mfa/recovery-codes/regenerate`

Regenerate all recovery codes (password required). Old codes are invalidated.

**Request:**
```json
{ "password": "CurrentP@ss1" }
```

**Response (200):**
```json
{
  "recovery_codes": ["abcd1234", "efgh5678", "..."],
  "message": "Recovery codes regenerated. Old codes are now invalid."
}
```

---

## Profile — Sessions 🔒

### GET `/profile/sessions`

List active sessions for the authenticated user. Current session is flagged.

**Response (200):**
```json
{
  "sessions": [
    {
      "id": "uuid",
      "ip_address": "192.168.1.1",
      "user_agent": "Mozilla/5.0...",
      "created_at": "2026-01-01T00:00:00Z",
      "last_active_at": "2026-01-01T12:00:00Z",
      "expires_at": "2026-01-02T00:00:00Z",
      "current": true
    }
  ]
}
```

---

### DELETE `/profile/sessions/{id}`

Revoke a specific session.

**Response (204):** No content.

---

### DELETE `/profile/sessions`

Revoke all sessions except the current one.

**Response (200):**
```json
{ "message": "All other sessions have been revoked" }
```

---

## Organizations 🔒

### POST `/orgs`

Create a new organization. The creator becomes the owner.

**Request:**
```json
{
  "name": "Acme Corp",
  "slug": "acme-corp",
  "plan": "free"
}
```

**Response (201):** Organization object

---

### GET `/orgs`

List the current user's organizations.

**Response (200):** Array of organization objects

---

### GET `/orgs/{slug}`

Get an organization by slug.

**Response (200):** Organization object  
**Errors:** `404` — Not found

---

## Feature Flags 🔒

### GET `/config/features`

List all feature flags.

**Response (200):**
```json
[
  {
    "name": "signup",
    "enabled": true,
    "description": "Allow new user registration"
  }
]
```

Current flags: `signup`, `invite_only`, `scan_docker_mode`, `public_api`

---

### GET `/config/auth`

Returns auth configuration. **Public endpoint** — no authentication required. Used by the UI to detect subdomain mode before the user has a session.

**Response (200):**
```json
{
  "base_domain": "aegis.io"
}
```

When `AEGIS_BASE_DOMAIN` is not set, `base_domain` is an empty string.

---

## MFA Enforcement (Org-Scoped Endpoints)

When an org admin enables the `require_mfa` org-level feature flag, all org-scoped endpoints (Scans, Findings, Projects, Members, Tokens, Dashboard, Org Features) will return **403** for users who haven't set up MFA:

```json
{
  "success": false,
  "request_id": "req_...",
  "errors": [
    {
      "type": "permission_error",
      "code": "mfa_required_by_org",
      "ref": "E60003",
      "message": "This organization requires MFA to be enabled"
    }
  ]
}
```

**Scope:** This check runs in the `TenantResolver` middleware, which handles all user-authenticated org-scoped requests. Agent (Bearer token) requests are **not affected** — they use the `TokenAuth` middleware instead.

**Resolution:** The user must set up MFA via their profile page (`/profile/access-management/authentication`), which is on the base domain and not org-scoped.

---

## Scans 🔒🏢 (Read-Only)

Requires org context (org subdomain or custom domain).

> **Note:** Scans are legacy data. New findings are pushed via the Agent Ingest API.

### GET `/scans`

List all scans in the current org (newest first).

**Response (200):** Array of scan objects

---

### GET `/scans/{id}`

Get scan details including finding counts and summary.

**Response (200):** Scan object  
**Errors:** `404` — Scan not found

---

## Findings 🔒🏢

### GET `/findings`

List findings with optional filters:

| Query Param | Description |
|---|---|
| `scan_id` | Filter by scan (legacy) |
| `project_id` | Filter by project |
| `severity` | `critical`, `high`, `medium`, `low`, `info` |
| `status` | `open`, `confirmed`, `fixed`, `false_positive`, `wontfix`, `verified` |
| `cwe` | Filter by CWE ID (e.g., `CWE-89`) |

**Response (200):** Array of finding objects

---

### GET `/findings/{id}`

Get full finding details.

**Response (200):** Finding object  
**Errors:** `404` — Finding not found

---

### PATCH `/findings/{id}`

Update finding status (triage).

**Request:**
```json
{ "status": "false_positive" }
```

**Valid statuses:** `open`, `confirmed`, `fixed`, `false_positive`, `wontfix`

**Response (200):** Updated finding object

---

### GET `/findings/{id}/exploits`

List PoC exploits for a finding.

**Response (200):** Array of exploit objects

---

### GET `/findings/{id}/exploits/{eid}`

Get a specific exploit with source code.

**Response (200):** Exploit object

---

## Dashboard 🔒🏢

### GET `/dashboard/stats`

Returns aggregate statistics for the current org.

**Response (200):**
```json
{
  "total_scans": 12,
  "active_scans": 2,
  "total_findings": 47,
  "severity_breakdown": {
    "total": 47,
    "critical": 3,
    "high": 8,
    "medium": 15,
    "low": 12,
    "info": 9
  },
  "recent_findings": [...]
}
```

---

## Projects 🔒🏢

### POST `/projects`

Create a project within the current org.

**Request:**
```json
{
  "name": "Backend API",
  "slug": "backend-api"
}
```

Slug is auto-generated from name if not provided. Must be at least 2 characters.

**Response (201):** Project object

---

### GET `/projects`

List projects in the current org.

**Response (200):** Array of project objects

---

### GET `/projects/{slug}`

Get project by slug.

**Response (200):** Project object  
**Errors:** `404` — Project not found

---

## Members 🔒🏢

### GET `/members`

List all members of the current org with their roles and profile info.

**Response (200):**
```json
[
  {
    "user_id": "uuid",
    "email": "user@example.com",
    "name": "John Doe",
    "avatar_url": "",
    "role": "owner"
  }
]
```

---

### POST `/members/invite`

Invite a user by email. If the user doesn't have an Aegis account, a placeholder account is created and they are added to the org.

**Request:**
```json
{
  "email": "new@example.com",
  "role": "member"
}
```

**Roles:** `owner`, `admin`, `member`, `viewer`

**Errors:**
- `400` — Missing email or invalid role
- `409` — User is already a member

---

### DELETE `/members/{userId}`

Remove a member from the org. Cannot remove yourself.

**Response:** `204 No Content`  
**Errors:**
- `400` — Cannot remove yourself
- `404` — Member not found

---

## Agent Ingest API 🔑

These endpoints use Bearer token authentication instead of cookies. Agents push findings one-at-a-time. Deduplication is based on `fingerprint`.

All agent endpoints require:
- `Authorization: Bearer aegis_xxx` header
- Org context via subdomain (`acme.aegis.io`) or custom domain

### POST `/agent/findings`

Push a finding. If a finding with the same `fingerprint` already exists, it updates `seen_count` and `last_seen_at` instead of creating a duplicate.

**Request:**
```json
{
  "scan_id": "0195xxxx-xxxx-7xxx-xxxx-xxxxxxxxxxxx",
  "project_id": "uuid",
  "fingerprint": "sha256:abc123def456...",
  "title": "SQL Injection in login handler",
  "severity": "critical",
  "cwe": "CWE-89",
  "cve": "CVE-2024-12345",
  "cvss_score": 9.8,
  "cvss_vector": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
  "file": "internal/auth/login.go",
  "line": 42,
  "description": "The login handler concatenates user input...",
  "source": "ci-run-456"
}
```

| Field | Required | Description |
|---|---|---|
| `scan_id` | Yes | UUID v7 scan correlation ID (max 36 chars). Generated by the agent at startup. |
| `project_id` | Yes | UUID of the project |
| `fingerprint` | Yes | Stable hash computed by agent (max 256 chars) |
| `title` | Yes | Short vulnerability title (max 500 chars) |
| `severity` | Yes | `critical`, `high`, `medium`, `low`, `info` |
| `description` | Yes | Markdown description (max 100KB) |
| `cwe` | No | CWE ID (format: `CWE-NNN`) |
| `cve` | No | CVE ID (format: `CVE-YYYY-NNNNN`) |
| `cvss_score` | No | CVSS score (0.0–10.0) |
| `cvss_vector` | No | CVSS vector string |
| `file` | No | File path (no `..` traversal) |
| `line` | No | Line number (for display, not dedup) |
| `source` | No | Grouping tag (e.g., CI run ID) |

**Response (201):** New finding + `{"deduplicated": false}`  
**Response (200):** Existing finding updated + `{"deduplicated": true}`

---

### GET `/agent/findings`

Pull findings for fix verification.

| Query Param | Required | Description |
|---|---|---|
| `project_id` | Yes | Filter by project |
| `status` | No | e.g., `fixed` |
| `severity` | No | Filter by severity |

**Response (200):** Array of finding objects

---

### PATCH `/agent/findings/{id}`

Update finding status. Agents can only set: `open` (reopen) or `verified` (confirm fix).

**Request:**
```json
{ "status": "verified" }
```

**Response (200):** Updated finding object

---

### POST `/agent/findings/{id}/exploits`

Attach a PoC exploit to a finding.

**Request:**
```json
{
  "filename": "exploit.py",
  "language": "python",
  "code": "import requests\n..."
}
```

**Response (201):** Exploit object

---

## Token Management — Project-Scoped 🔒🏢

Tokens scoped to a specific project. Any org member can manage project tokens.

### POST `/projects/{projectId}/tokens`

Generate a new API token scoped to a specific project. The plaintext token is returned **once** and never stored.

**Request:**
```json
{
  "name": "CI Pipeline Token",
  "expires_in": 90
}
```

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Display name (max 100 chars) |
| `expires_in` | No | Days until expiry (0 = never) |

**Response (201):**
```json
{
  "token": "aegis_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6",
  "info": {
    "id": "uuid",
    "project_id": "project-uuid",
    "name": "CI Pipeline Token",
    "prefix": "aegis_a1b2c3d4",
    "created_by": "user-uuid",
    "expires_at": "2026-09-11T00:00:00Z",
    "revoked": false,
    "created_at": "2026-06-11T00:00:00Z"
  }
}
```

> ⚠️ Save the `token` value immediately — it cannot be retrieved again.

---

### GET `/projects/{projectId}/tokens`

List all tokens for a specific project (prefix and metadata only, never the full token).

**Response (200):** Array of token objects

---

### DELETE `/projects/{projectId}/tokens/{id}`

Revoke a project-scoped token. The token must belong to the specified project.

**Response:** `204 No Content`

**Errors:**
- `403` — Token does not belong to this project
- `404` — Token not found

---

## Token Management — Org-Wide 🔒🏢

Org-wide tokens have access to all projects. Requires **admin or owner** role. Gated by the `org_wide_tokens` org feature flag.

### POST `/tokens`

Generate an org-wide API token. Requires `org_wide_tokens` feature to be active.

**Request:**
```json
{
  "name": "CI Pipeline Token",
  "expires_in": 90
}
```

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Display name (max 100 chars) |
| `expires_in` | No | Days until expiry (0 = never) |

**Response (201):** Same format as project token response, but `project_id` is empty.

**Errors:**
- `403` — `permission_error.denied` — Admin/owner role required
- `403` — `permission_error.feature_disabled` — `org_wide_tokens` feature not enabled

---

### GET `/tokens`

List all org tokens (project-scoped and org-wide). Admin/owner only.

**Response (200):** Array of token objects

---

### DELETE `/tokens/{id}`

Revoke an org token. Admin/owner only.

**Response:** `204 No Content`

---

## Org Feature Flags 🔒🏢

Per-org feature flags use a two-layer model:
- **provisioned**: Set by platform admins — determines if the org can see the flag at all
- **enabled**: Set by org owner — toggles the feature on/off

A feature is **active** only when both `provisioned` AND `enabled` are `true`.

### GET `/org-features`

List all org-level feature flags. Any org member can read.

**Response (200):**
```json
[
  {
    "name": "org_wide_tokens",
    "provisioned": true,
    "enabled": false,
    "description": "Allow creating org-wide API tokens that access all projects"
  }
]
```

---

### PATCH `/org-features/{flag}`

Toggle the enabled state of an org feature flag. **Owner only.** The flag must be provisioned.

**Request:**
```json
{ "enabled": true }
```

**Response (200):**
```json
{ "message": "feature flag updated" }
```

**Errors:**
- `403` — `permission_error.denied` — Only the org owner can toggle flags
- `403` — `permission_error.feature_not_provisioned` — Flag not found or not provisioned

---

## Health

### GET `/healthz`

Liveness probe. Pings the database to verify connectivity.

**Response (200):**
```json
{
  "status": "ok",
  "db": "ok",
  "timestamp": "2026-06-11T10:00:00Z"
}
```

**Response (503):** When database is unreachable:
```json
{
  "status": "degraded",
  "db": "unreachable",
  "timestamp": "2026-06-11T10:00:00Z"
}
```

### GET `/readyz`

Readiness probe. Same behavior as `/healthz`.

---

## Metrics

### GET `/metrics`

Prometheus metrics endpoint. Returns metrics in Prometheus text exposition format.

**Metrics exported:**

| Metric | Type | Description |
|---|---|---|
| `aegis_http_requests_total` | counter | Total HTTP requests (by method, path, status) |
| `aegis_http_request_duration_seconds` | histogram | Request duration (by method, path) |
| `aegis_http_active_requests` | gauge | Currently in-flight requests |
| `aegis_db_pool_open_connections` | gauge | Open database connections |
| `aegis_db_pool_in_use` | gauge | Connections currently in use |
| `aegis_db_pool_idle` | gauge | Idle connections |

---

## API Documentation

### GET `/api/v1/docs`

Interactive Swagger UI for exploring the API.

### GET `/api/v1/docs/openapi.yaml`

Raw OpenAPI 3.0.3 specification in YAML format.

---

## Legend

- 🔒 — Requires authentication (`aegis_token` cookie)
- 🏢 — Requires org context (org subdomain or custom domain)
- 🔑 — Requires agent token (`Authorization: Bearer aegis_xxx`)
