# Aegis Roadmap

Last updated: 2026-06-12

This document tracks planned features, improvements, and technical debt for the Aegis platform. Items are organized by priority and grouped into phases.

---

## Phase 1 — Core UI Completeness

High-priority items to make the existing platform fully functional end-to-end.

### Token Management UI
- **Status:** ✅ Completed (2026-06-11)
- **Priority:** 🔴 Critical
- **Description:** API Tokens tab in Settings page for managing agent authentication tokens.
- **Implemented:**
  - Token creation dialog (name, project scope, expiry)
  - Show plaintext token once after creation (with copy button + security warning)
  - List tokens with prefix, name, project scope, expiry, status (active/expired/revoked)
  - Revoke button per token
  - `tokensApi` client (create, list, revoke)

### Finding Detail Enhancements
- **Status:** ✅ Completed (2026-06-11)
- **Priority:** 🔴 Critical
- **Description:** Finding detail page displays agent-pushed metadata: CVE, CVSS score (color-coded), seen count, last seen at, verification status.
- **Implemented:**
  - CVE display in sidebar details
  - CVSS score with color indicator (red ≥9.0, orange ≥7.0, amber ≥4.0, blue <4.0)
  - `seen_count` display ("Seen N times") when >1 for dedup visibility
  - `last_seen_at` timestamp when different from created_at
  - `verified` status added to findings list filter dropdown
  - Project filter dropdown on findings list page

### Org Creation UI
- **Status:** ✅ Completed (2026-06-12)
- **Priority:** 🔴 Critical
- **Description:** Users can create new organizations from the sidebar org switcher. The backend existed but the UI had no dialog wired to it.
- **Implemented:**
  - Create Organization dialog (name + auto-generated slug + slug preview)
  - Slug validation (3-50 chars, lowercase alphanumeric + hyphens, reserved slug check)
  - Live subdomain preview (`slug.aegis.io`)
  - Error display for duplicate slugs, reserved names, and validation failures
  - Backend: improved error handling (409 Conflict for reserved/duplicate slugs, plan allowlist validation)
  - Org switcher "Create Organization" button wired to dialog
  - Auto-switch to new org after creation
  - "No Organization" empty state now clickable to open dialog

---

## Phase 2 — Reports & Analytics

Fill in the sidebar sections that exist as navigation items but have no implementation.

### Reports Page
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** `/reports` route is in the sidebar nav but has no page.
- **Requirements:**
  - Generate PDF/HTML security reports for a scan
  - Executive summary with severity charts
  - Detailed findings list with remediation guidance
  - Export to PDF

### Analytics Page
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** `/analytics` route is in the sidebar nav but has no page.
- **Requirements:**
  - Trends over time (findings opened vs closed)
  - Severity distribution charts
  - Mean time to remediation
  - Per-scan comparison view
  - Needs new API endpoints for time-series data

---

## Phase 3 — Project Integration

### Link Scans to Projects
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Projects exist as an entity but scans have no `project_id` association.
- **Changes needed:**
  - Add `project_id` column to `scans` table (migration)
  - Update `Scan` model, store, and API handlers
  - Filter scans by project in the UI
  - Project detail page showing its scans and aggregate findings

### Project Settings
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Per-project configuration (default persona, auto-scan schedule, notification preferences).

---

## Phase 4 — Org-Level Feature Flags & Versioning

Introduce per-org feature control and org versioning to allow independent feature rollouts and graduated upgrades across tenants.

### Org-Level Feature Flags
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** Create a per-org feature flag table (`org_xxx.org_feature_flags`) separate from the global `common.feature_flags`. This allows org admins to toggle features per-org and enables platform operators to grant/deny specific features to individual orgs.
- **Design:**
  - **Two-tier system:** Global flags in `common.feature_flags` act as a kill-switch. Org flags in `org_xxx.org_feature_flags` provide per-org overrides. A feature is enabled only if **both** the global flag AND the org flag are enabled (logical AND).
  - **Schema:** `org_xxx.org_feature_flags` table with columns: `name TEXT PK`, `enabled BOOLEAN`, `description TEXT`, `updated_at TIMESTAMPTZ`, `updated_by UUID` (tracks who changed it).
  - **Default seeding:** On org provisioning, seed default org flags based on the org's plan (e.g., free orgs get fewer flags enabled).
  - **API:** CRUD endpoints at `GET/PUT /api/v1/org-features` (protected, admin+ role).
  - **UI:** Settings page tab for org admins to view and toggle org-level features.
- **Example org-level flags:**
  - `require_mfa` — Users must have MFA enabled to access the org
  - `ip_restrictions` — Enable IP allowlist enforcement for the org
  - `email_domain_restriction` — Only allow users from specific email domains to join
  - `auto_join` — Auto-join users based on email domain match
  - `api_access` — Allow API token creation for this org
  - `scan_docker_mode` — Enable Docker sandbox mode for this org's scans
  - `advanced_reports` — Enable advanced/PDF reporting features
  - `webhooks` — Enable webhook integrations
  - `sso` — Enable SSO/SAML login for the org
  - `custom_domain` — Allow custom domain configuration

### Org Versioning
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** Track a `schema_version` per org so tenants can be on different schema versions. This allows rolling upgrades where org A stays on v1 while org B is upgraded to v2.
- **Design:**
  - Add `schema_version INTEGER DEFAULT 1` to `common.organizations`.
  - Each org schema migration records which version it applies. The migration runner checks the org's current version and only applies migrations above that version.
  - **Graduated rollout:** New migrations can target specific orgs or org plans first (e.g., upgrade `enterprise` orgs to v2 first, then `pro`, then `free`).
  - **Rollback safety:** Each migration version is immutable once applied. Rollback requires a separate "down" migration at a higher version number.
  - **Version-gated features:** Org-level feature flags can reference a minimum schema version. If an org hasn't been migrated to the required version, the flag is force-disabled regardless of its setting.
- **Changes needed:**
  - Add `schema_version` column to `common.organizations` (migration)
  - Add `org_schema_migrations` table to each org schema to track per-org migration history
  - Update `ProvisionOrgSchema()` to stamp the latest version
  - Create a migration runner that operates per-org (not globally)
  - Admin API to check org version and trigger upgrades
  - **Feature-version coupling:** Org feature flags include an optional `min_version` field — the feature is auto-disabled if the org's schema version is below this threshold

---

## Phase 5 — Org Policies & Access Control

Enterprise-grade controls for managing who can access an org and how.

### Two-Level Role-Based Access Control (RBAC)
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** Implement a two-tier RBAC system with **org-level roles** (controlling org-wide access) and **project-level roles** (controlling per-project access). Roles exist in the data model (`owner`, `admin`, `member`, `viewer`) but are not enforced in any handler today. This design is inspired by Langfuse's RBAC model, adapted for Aegis's security scanning domain.

#### Design Overview

Aegis RBAC operates at two scopes:

1. **Org-level role** — set in `common.org_members.role`. Determines default access to all projects in the org and controls org-wide resources (members, feature flags, billing, audit logs).
2. **Project-level role** — set in `org_<uuid>.project_members`. Overrides the org-level default for a specific project. Enables granting access to individual projects without org-wide visibility.

**Resolution logic:**
```
effective_access(user, project) =
    project_role(user, project)    ← if explicitly assigned
    ?? org_role(user)              ← fallback to org-level default
```

- Org `Owner` → gets Owner-level access to **all** projects by default (unless overridden)
- Org `None` + Project `Member` on "Backend API" → can only see that one project's findings/scans
- Org `Member` + Project `Admin` on "Mobile App" → elevated access on that specific project

#### Org Roles: `owner`, `admin`, `member`, `viewer`, `none`

| | **Owner** | **Admin** | **Member** | **Viewer** | **None** |
|---|---|---|---|---|---|
| **Organization** | CRUD, delete, transfer | update | — | — | — |
| **Org Members** | CUD, read | CUD, read | read | — | — |
| **Projects** | create, delete | create | — | — | — |
| **Org Feature Flags** | CUD, read | CUD, read | — | — | — |
| **Billing / Plan** | CRUD | read | — | — | — |
| **Audit Logs** | read | read | — | — | — |
| **API Tokens (org-wide)** | CUD, read | CUD, read | — | — | — |

- **Owner**: Full control — org deletion, ownership transfer, billing, all resources.
- **Admin**: Manage members, projects, tokens, feature flags. Cannot delete org or transfer ownership.
- **Member**: Read-only view of org members. Primary access is through project-level scopes (see below).
- **Viewer**: No org-level scopes. Read-only access to projects they can see (all projects, since org role is the default floor).
- **None**: No org-wide access at all. User must be explicitly granted project-level roles to access anything. Useful for external contractors, auditors, or per-team scoping.

#### Project-Level Scopes (inherited from org role as default)

When a user has an org role but no explicit project role, the org role determines their default project-level access across **all** projects:

| | **Owner** | **Admin** | **Member** | **Viewer** | **None** |
|---|---|---|---|---|---|
| **Project Settings** | read, update, delete | read, update | read | read | — |
| **Project Members** | CUD, read | CUD, read | read | — | — |
| **Findings** | CUD, read | CUD, read | CUD, read | read | — |
| **Scans** | CUD, read, delete | CUD, read | create, read | read | — |
| **Exploits** | CUD, read | CUD, read | read | read | — |
| **API Tokens (project-scoped)** | CUD, read | CUD, read | read | — | — |
| **Dashboard / Stats** | read | read | read | read | — |

Key notes:
- **Member** can create scans and triage findings (CUD on findings = change status, add notes) but cannot delete scans or manage project members.
- **Viewer** is strictly read-only across all project resources.
- **None** gets zero access unless a project-level role is explicitly assigned.

#### Project-Level Roles (TBD)

Project-level roles allow overriding the org-level default for a specific project. The exact project roles and their scopes will be defined in a future iteration.

**Expected roles:** `admin`, `member`, `viewer` (no `owner` or `none` at project level — those are org-level concepts).

**Expected storage:** `org_<uuid>.project_members` table:
```sql
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);
```

#### Implementation Requirements

**Data model changes:**
- Add `none` as a valid org role in `common.org_members.role`
- Create `org_<uuid>.project_members` table (new table in org schema)
- Update `ProvisionOrgSchema()` to include `project_members` table
- Add migration for existing org schemas

**Backend (Go server):**
- **RBAC middleware or per-handler checks** — extract user's org role from `common.org_members`, check project role from `project_members` if applicable, compute effective role
- **Scope-checking helper:** `hasPermission(ctx, resource, action) bool` — used in each handler to gate access
- Define permissions as constants: `PermFindingsRead`, `PermFindingsCUD`, `PermMembersRead`, `PermMembersCUD`, `PermProjectsCreate`, `PermOrgDelete`, etc.
- Map each role to its set of permissions (org-level and project-level separately)
- Return `403 Forbidden` with `{"error": "insufficient permissions"}` when a user lacks the required scope

**Frontend (React UI):**
- Fetch user's effective role via `GET /api/v1/auth/me` (org role) and project context
- Expose role in `useAuth()` or `useOrg()` context
- Hide/disable UI elements based on role (e.g., hide "Create Project" button for `member`, hide "Delete" for `viewer`)
- Role dropdown in member management (Settings → Members tab) with tooltip showing what each role can do (like Langfuse's pattern)

**API changes:**
- `GET /api/v1/members` — include role in response (already done)
- `PATCH /api/v1/members/{userId}` — change a member's org role (new endpoint, admin+ only)
- `GET /api/v1/projects/{slug}/members` — list project members with roles (new)
- `POST /api/v1/projects/{slug}/members` — add project member with role (new)
- `PATCH /api/v1/projects/{slug}/members/{userId}` — change project role (new)
- `DELETE /api/v1/projects/{slug}/members/{userId}` — remove project member (new)

### MFA Enforcement
- **Status:** ✅ Completed (2026-06-11)
- **Priority:** 🟡 Medium
- **Description:** Multi-device MFA with explicit user toggle. Users can add TOTP authenticator apps and email OTP as second factors. MFA is controlled via `mfa_enabled` on `common.users` as a manual toggle.
- **Implemented:**
  - Multi-device MFA: TOTP (Google Authenticator, Authy, etc.) and email OTP
  - Device management: add, verify, remove (password required)
  - MFA enable/disable is an explicit user action (not auto-computed)
  - Enable prerequisite: at least one verified TOTP device OR verified email
  - 8 bcrypt-hashed recovery codes (one-time use, regeneratable)
  - Multi-method login challenge: device selector, email OTP sending
  - Org-level `require_mfa` flag: if enabled, `TenantResolver` middleware blocks access with 403 if user has MFA disabled
  - Profile page with 3 tabs: Emails, Authentication, Active Sessions
  - Session tracking via JTI in JWT — list, revoke, revoke all
- **Future:**
  - Hardware keys (FIDO2/WebAuthn — YubiKey, passkeys)

### IP Restriction
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Allow orgs to restrict access to specific IP addresses or CIDR ranges. Controlled via the `ip_restrictions` org-level feature flag.
- **Requirements:**
  - `org_xxx.ip_allowlist` table: `id UUID PK`, `cidr TEXT NOT NULL`, `description TEXT`, `created_by UUID`, `created_at TIMESTAMPTZ`
  - API endpoints: `GET/POST/DELETE /api/v1/ip-restrictions` (admin+ role)
  - Middleware check: if `ip_restrictions` flag is enabled, validate request IP against allowlist before granting access
  - Applies to both user (cookie) and agent (bearer token) requests
  - UI: Settings page tab for managing IP allowlist entries
  - Support IPv4 and IPv6 CIDR notation

### Email Domain Restriction
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Allow orgs to restrict which email domains can join. Controlled via the `email_domain_restriction` org-level feature flag.
- **Requirements:**
  - `org_xxx.allowed_email_domains` table: `domain TEXT PK`, `created_at TIMESTAMPTZ`
  - When inviting a user or accepting a join request, validate email domain against the allowlist
  - API endpoints: `GET/POST/DELETE /api/v1/email-domains` (admin+ role)
  - UI: Settings page tab for managing allowed email domains

### Auto-Join by Email Domain
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Automatically add users to an org if their email domain matches a configured domain. Controlled via the `auto_join` org-level feature flag.
- **Requirements:**
  - `org_xxx.auto_join_domains` table: `domain TEXT PK`, `default_role TEXT DEFAULT 'member'`, `created_at TIMESTAMPTZ`
  - On user registration or first login, check if any org has an auto-join domain matching the user's email — if so, auto-add as member with the configured default role
  - API endpoints: `GET/POST/DELETE /api/v1/auto-join-domains` (owner role only)
  - UI: Settings page tab for configuring auto-join domains and default role

### Audit Log
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** Log security-relevant actions (member invite/remove, scan create/delete, finding triage changes, feature flag changes, IP allowlist changes, login/logout, password changes, MFA changes) for compliance. Essential for a security platform.
- **Changes needed:**
  - `audit_log` table in per-org schema: `id UUID`, `actor_id UUID`, `action TEXT`, `resource_type TEXT`, `resource_id UUID`, `metadata JSONB`, `ip_address TEXT`, `user_agent TEXT`, `created_at TIMESTAMPTZ`
  - `common.auth_audit_log` table for auth events (login, logout, password changes, MFA changes) — not org-scoped
  - Store interface + postgres implementation
  - Write audit entries from handlers (non-blocking via goroutines)
  - UI page to browse and filter audit log (by actor, action, date range)
  - Export to CSV/JSON for compliance reports

### Session Management
- **Status:** ✅ Completed (2026-06-11)
- **Priority:** 🟢 Low
- **Description:** Users can view and revoke active sessions from Profile → Active Sessions tab. Sessions store browser, OS, device type (parsed from User-Agent server-side). Valkey cache layer reduces DB load on session revocation checks.
- **Implemented:**
  - `common.user_sessions` table with JTI, IP, user agent, browser, OS, device type
  - Profile page: list sessions, revoke individual, revoke all others
  - Valkey cache for session revocation checks (5-min TTL, graceful fallback to DB)
- **Future:**
  - Org admin action: "Force logout all members" (revokes all sessions for users in the org)

---

## Phase 6 — Integrations

### CI/CD Integration Guide
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Provide ready-to-use GitHub Action, GitLab CI template, and generic CLI examples for pushing findings from CI pipelines.
- **Requirements:**
  - Example GitHub Action workflow YAML
  - Example GitLab CI job config
  - `curl`-based examples for any CI system
  - Document token scoping per project

### Webhook Notifications
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Allow orgs to configure webhooks that fire when findings are created, status changes, or agents complete a scan pass. Controlled via the `webhooks` org-level feature flag.
- **Requirements:**
  - `org_xxx.webhooks` table (URL, events, secret)
  - HMAC-signed POST to webhook URL
  - Retry with exponential backoff

### SSO / SAML Integration
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Allow enterprise orgs to configure SAML-based SSO. Controlled via the `sso` org-level feature flag.
- **Requirements:**
  - SAML 2.0 SP implementation
  - `org_xxx.sso_config` table (entity_id, sso_url, certificate, etc.)
  - Login flow: redirect to IdP → callback → create/link user → set session
  - Support for Okta, Azure AD, Google Workspace, OneLogin
  - Auto-provisioning of users from IdP attributes

### Slack / Teams Notifications
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Send scan completion and critical finding alerts to Slack or Microsoft Teams channels.
- **Requirements:**
  - `org_xxx.notification_channels` table (type, webhook_url, events, enabled)
  - Slack incoming webhook integration
  - Teams incoming webhook integration
  - Configurable event filters (severity threshold, scan status changes)

---

## Phase 6.5 — Advanced Security

Hardening features for session integrity, threat detection, and token lifecycle.

### Token Refresh Rotation
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Replace the current single long-lived JWT with a short-lived access token + long-lived refresh token pair. Refresh tokens are rotated on each use (one-time use), preventing replay attacks if a token is stolen.
- **Requirements:**
  - Short-lived access token (15 min) + refresh token (7 days) pair
  - `common.refresh_tokens` table: `id UUID PK`, `user_id UUID`, `token_hash TEXT`, `family TEXT` (rotation chain), `expires_at TIMESTAMPTZ`, `revoked_at TIMESTAMPTZ`
  - `POST /api/v1/auth/refresh` endpoint: validate refresh token, issue new access+refresh pair, revoke old refresh token
  - **Rotation detection:** If a revoked refresh token is reused, revoke the entire token family (all descendants) — this indicates theft
  - Update frontend `request()` helper to auto-refresh on 401
  - Graceful migration: existing sessions continue to work until expiry

### Suspicious Activity Detection
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Detect and alert on potentially malicious login patterns: new device/location, impossible travel, brute-force attempts, and credential stuffing.
- **Requirements:**
  - **New location/device alert:** Compare login IP + UA against user's session history. If new, send an email alert (partially done — login notification emails already sent on every login).
  - **Impossible travel detection:** If two logins from geographically distant locations within a short time window, flag the newer session and notify the user.
  - **Brute-force detection:** Track failed login attempts per IP and per account in Valkey. Lock out after N failures (e.g., 5 failures → 15-min lockout).
  - **Credential stuffing detection:** Track failed login rate per IP across multiple accounts. If an IP fails against many different accounts, block the IP temporarily.
  - **GeoIP integration:** Add MaxMind GeoLite2 (or ip-api) for IP-to-location resolution. Store country/city in `user_sessions`.
  - `common.security_events` table: `id UUID`, `user_id UUID`, `event_type TEXT`, `ip TEXT`, `metadata JSONB`, `created_at TIMESTAMPTZ`
  - Admin dashboard widget showing recent security events
  - Org-level feature flag: `suspicious_activity_detection`

### Password Policy Enforcement
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Strengthen password validation beyond minimum length. Check passwords against the HaveIBeenPwned breached passwords database (k-anonymity API — only sends a 5-char hash prefix, never the full password).
- **Requirements:**
  - Integrate HIBP Pwned Passwords API (k-anonymity model: `GET https://api.pwnedpasswords.com/range/{prefix}`)
  - Reject passwords that appear in known breaches during registration, password change, and password reset
  - Configurable minimum complexity rules (uppercase, digit, special char) — enforce via org-level feature flag `strict_password_policy`
  - User-facing error: "This password has appeared in a data breach. Please choose a different password."

### Session Binding
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Bind sessions to the originating IP address and User-Agent. If a session cookie is used from a different IP or device, require re-authentication. This prevents stolen cookie replay attacks.
- **Requirements:**
  - Store `ip_address` and `user_agent` hash in `common.user_sessions` (already stored — need enforcement)
  - On each authenticated request, compare current IP + UA against the session's stored values
  - **Strict mode** (org flag `strict_session_binding`): mismatch → session revoked, user forced to re-login
  - **Soft mode** (default): mismatch → log a security event + send email alert, but allow the request
  - Handle legitimate IP changes (mobile networks, VPNs) gracefully — soft mode is the safe default

### API Rate Limiting
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** Rate limit auth endpoints (login, register, password reset, MFA) and agent ingest endpoints to prevent brute-force and abuse. Use Valkey for distributed counters.
- **Requirements:**
  - **Auth endpoints:** 5 attempts per minute per IP for login/register, 3 per hour for password reset
  - **Agent endpoints:** Per-org token bucket (free: 100 req/min, pro: 1000, enterprise: 10000)
  - **Global:** 1000 req/min per IP across all endpoints
  - Valkey-backed sliding window counters
  - `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` response headers
  - `429 Too Many Requests` with `Retry-After` header
  - Rate limit middleware in the request chain (before auth, for auth endpoints)

### Content Security Policy
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Add a `Content-Security-Policy` header to all responses to mitigate XSS, clickjacking, and data injection attacks.
- **Requirements:**
  - `default-src 'self'` — restrict all resources to same-origin by default
  - `script-src 'self'` — no inline scripts, no eval
  - `style-src 'self' 'unsafe-inline'` — allow inline styles (needed for shadcn/ui)
  - `img-src 'self' data:` — allow data URIs for avatars
  - `connect-src 'self'` — restrict fetch/XHR to same-origin
  - `frame-ancestors 'none'` — prevent framing (supplements X-Frame-Options)
  - Add CSP header in `ServeHTTP` alongside existing security headers
  - Consider `report-uri` / `report-to` for monitoring violations in production

### Cookie Security Hardening
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Harden auth cookies with `Secure` flag and `__Secure-` prefix once HTTPS is enforced in production. Currently cookies use `HttpOnly`, `SameSite=Lax`, and `Path=/` but lack `Secure: true` (which would break local HTTP dev).
- **Requirements:**
  - Add `Secure: true` to all auth cookies (requires HTTPS)
  - Rename cookie from `aegis_token` to `__Secure-aegis_token` (browser enforces Secure + no Domain override)
  - Note: `__Host-` prefix cannot be used because it disallows `Domain` attribute, which is required for cross-subdomain cookie sharing
  - Consider making Secure flag configurable via env var for dev vs prod
  - Add CSRF token protection for all mutating cookie-authenticated endpoints (see CSRF Token Protection item)

### Password History
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Prevent users from reusing recent passwords when changing or resetting their password. Stores hashed versions of previous passwords.
- **Requirements:**
  - `common.password_history` table: `id UUID PK`, `user_id UUID`, `password_hash TEXT`, `created_at TIMESTAMPTZ`
  - On password change/reset, check new password against last N (default: 5) stored hashes via bcrypt
  - Configurable depth via org-level feature flag `password_history_depth` (default: 5)
  - Automatically prune history entries beyond the configured depth
  - User-facing error: "You cannot reuse any of your last 5 passwords."

### Account Lockout
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** Lock accounts after N consecutive failed login attempts to prevent brute-force password guessing. Currently there is no lockout — unlimited attempts are allowed.
- **Requirements:**
  - Track failed login attempts per account in Valkey: `login_failures:{user_id}` counter with TTL
  - After 5 consecutive failures → lock account for 15 minutes
  - After 10 consecutive failures → lock account for 1 hour
  - After 20 consecutive failures → lock account until manual unlock or password reset
  - Reset counter on successful login
  - Send email notification to user when account is locked
  - `common.account_lockouts` table: `user_id UUID PK`, `locked_until TIMESTAMPTZ`, `attempt_count INT`, `last_attempt_at TIMESTAMPTZ`
  - Admin API to manually unlock accounts
  - User-facing error: "Account temporarily locked. Try again in X minutes or reset your password."

### MFA Brute-Force Protection
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** OTP codes (6 digits) can currently be guessed endlessly with no attempt limit. An attacker with a valid MFA token can try all 1,000,000 combinations.
- **Requirements:**
  - Track MFA verification attempts per MFA token in Valkey: `mfa_attempts:{mfa_token}` counter with TTL matching MFA token expiry
  - After 3 failed attempts → invalidate the MFA token entirely, force user to re-enter password
  - After 5 failed attempts (across multiple MFA tokens) within 10 minutes → trigger account lockout
  - Rate limit `POST /api/v1/auth/mfa/validate` to 1 request per 2 seconds per IP
  - Rate limit `POST /api/v1/auth/mfa/send-email-otp` to 1 request per 30 seconds per MFA token
  - Log all failed MFA attempts to audit log
  - User-facing error: "Too many failed attempts. Please log in again."

### CSRF Token Protection
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Currently relying solely on `SameSite=Lax` cookies for CSRF protection. While `SameSite=Lax` prevents most CSRF attacks, it does not protect against top-level navigation attacks (GET-based state changes) and some browser edge cases.
- **Requirements:**
  - Generate a CSRF token on login and store in session (Valkey or DB)
  - Return token via a `GET /api/v1/auth/csrf` endpoint or as a response header
  - Frontend stores token in memory (not localStorage) and sends it as `X-CSRF-Token` header on all mutating requests (POST/PATCH/DELETE)
  - Server middleware validates `X-CSRF-Token` matches session token on all mutating requests
  - Exempt agent API (Bearer token auth) from CSRF checks — CSRF only applies to cookie-based auth
  - Token rotation: regenerate on sensitive actions (password change, MFA toggle)
  - Double-submit cookie pattern as fallback for non-SPA clients

---

## Phase 7 — User Experience

### Notifications System
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** The sidebar user menu has a "Notifications" item but nothing exists. Notify users when scans complete or critical findings are discovered.

### User Account Page
- **Status:** ✅ Completed (2026-06-11)
- **Priority:** 🟢 Low
- **Description:** Profile page with 3 tabs: Emails (multi-email management, verification), Authentication (password change, multi-device MFA, recovery codes), Active Sessions (list, revoke, revoke all).

### Search / Command Palette
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** The sidebar has a "Search" button at the bottom. Implement a global search / command palette (Cmd+K) to quickly navigate to scans, findings, or settings.

### Dark Mode Toggle
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** The UI uses shadcn/ui which supports dark mode theming. Add a toggle in settings or the user menu.

---

## Phase 8 — Platform & Enterprise Features

### Org Plan Limits
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Enforce usage limits based on org plan (free, pro, enterprise).
- **Requirements:**
  - Define limits per plan: max members, max projects, max scans/month, max API tokens, max findings retention
  - `common.plan_limits` table or hardcoded config
  - Enforce in API handlers (return 402/403 when limit exceeded)
  - UI banner showing usage vs. limit

### Data Retention Policies
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Allow orgs to configure how long findings and scan data are retained before automatic cleanup.
- **Requirements:**
  - `data_retention_days` setting in org feature flags or org settings table
  - Background job to purge expired data
  - UI configuration in org settings

### Org Branding / White Label
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Allow enterprise orgs to customize the UI with their logo, color scheme, and custom domain.
- **Requirements:**
  - `org_xxx.branding` table (logo_url, primary_color, accent_color)
  - Dynamic theme loading based on org context
  - Custom email templates with org branding

### API Rate Limiting (Per-Org)
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Per-org rate limits on API and agent endpoints, configurable via org feature flags.
- **Requirements:**
  - Token bucket or sliding window rate limiter
  - Different limits per plan (free: 100 req/min, pro: 1000, enterprise: unlimited)
  - `X-RateLimit-*` response headers
  - 429 Too Many Requests response with `Retry-After`

### Multi-Region / Data Residency
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Allow orgs to choose their data region for compliance (GDPR, SOC2, etc.).
- **Requirements:**
  - `data_region` column on `common.organizations`
  - Route org queries to region-specific database
  - Region selector during org creation

---

## Technical Debt

| Item | Description | Priority |
|---|---|---|
| ~~Standardized API response format~~ | ~~No consistent response envelope — each endpoint invents its own shape; no pagination on list endpoints~~ | ✅ Completed |
| ~~Structured error codes~~ | ~~All API errors return `{"error": "message"}` — no machine-readable codes for programmatic handling~~ | ✅ Completed |
| Error boundaries | No React error boundaries — unhandled errors crash the whole app | 🟡 Medium |
| Loading states | Some pages show raw empty states before data loads | 🟢 Low |
| Test suite | No unit tests or integration tests exist | 🟡 Medium |
| Rate limiting | No rate limiting on auth or agent ingest endpoints | 🟡 Medium |
| OpenAPI spec update | Swagger spec needs agent ingest + token endpoints added | 🟡 Medium |
| Feature flag consistency | Migrate all feature-gated behavior to use the two-tier flag system (global + org) | 🟡 Medium |
| ~~Request trace IDs~~ | ~~No request ID generated per request — logs, errors, and API responses can't be correlated for debugging~~ | ✅ Completed |

### Standardized API Response Envelope & Pagination
- **Status:** ✅ Completed (2026-06-12)
- **Priority:** 🔴 Critical
- **Description:** All API endpoints now use a Cloudflare-style response envelope with consistent success/error formatting.
- **Implemented:**
  - Standard success envelope: `{"success": true, "request_id": "req_...", "result": {...}}`
  - Standard error envelope: `{"success": false, "request_id": "req_...", "errors": [{"type": "...", "code": "...", "ref": "E...", "message": "..."}]}`
  - Helpers: `writeResult`, `writeResultMessage`, `writeList`, `writeMessage`, `writeApiError`, `writeValidationErrors`
  - Pagination infrastructure: `ResultInfo` struct, `parseResultInfo` query parser, `writeList` for paginated responses
  - Frontend: `ApiError` class, automatic envelope unwrapping in `request()`, `requestList()` helper
  - All ~250 handler call sites migrated from legacy `writeJSON`/`writeError` to new helpers
  - Legacy `writeJSON`/`writeError` functions deprecated (no callers remain)
- **Not yet implemented:**
  - Store-level pagination (paginated SQL queries with `COUNT(*) OVER()`) — deferred, infrastructure is ready
  - `go generate` codegen script for error registry — deferred

### Structured Error Codes
- **Status:** ✅ Completed (2026-06-12)
- **Priority:** 🔴 Critical
- **Description:** All API errors now return structured error codes with type, code, reference ID, and message.
- **Implemented:**
  - Master error registry: `errors.yaml` (single source of truth)
  - Go error constants: `server/internal/api/errors.go` (30+ pre-defined `ApiError` constants)
  - TypeScript error constants: `ui/src/lib/error-codes.gen.ts` (matching frontend constants)
  - Error code categories: `auth_error` (E1xxxx), `token_error` (E2xxxx), `tenant_error` (E3xxxx), `resource_error` (E4xxxx), `validation_error` (E5xxxx), `permission_error` (E6xxxx), `rate_limit_error` (E7xxxx), `server_error` (E9xxxx)
  - `WithMessage()` and `WithDetails()` builder methods on `ApiError`
  - Field-level validation errors via `writeValidationErrors()` with `FieldError` details
  - Frontend `ApiError` class with error code, type, ref, and message

### Request Trace IDs
- **Status:** ✅ Completed (2026-06-12)
- **Priority:** 🔴 Critical
- **Description:** Every API request now has a unique request ID propagated through logs, error responses, and response headers.
- **Implemented:**
  - Request ID middleware (`server/internal/middleware/request_id.go`) — generates `req_<hex>` IDs
  - Request ID context utilities (`server/internal/requestid/requestid.go`)
  - `X-Request-ID` response header on all responses
  - Request ID in all error response bodies (`request_id` field)
  - Request ID in all success response bodies (`request_id` field)
  - Logger integration: `request_id` automatically injected into all `slog` output
  - Accepts incoming `X-Request-ID` from reverse proxies
  - Middleware is outermost in the chain (runs before auth, CORS, etc.)

---

## Completed ✅

| Feature | Date |
|---|---|
| Auth system (register, login, logout, JWT sessions) | Done |
| Multi-tenant architecture (per-org PostgreSQL schemas) | Done |
| Organization CRUD + auto-provisioning | Done |
| Schema migration system | Done |
| Feature flags (global) | Done |
| Finding CRUD + triage status | Done |
| Exploit storage + display | Done |
| Dashboard with aggregate stats | Done |
| Project CRUD | Done |
| Member invite/remove | Done |
| Login page | Done |
| Sidebar navigation + org/project switcher | Done |
| Finding detail page with markdown rendering | Done |
| Settings page (General + Members tabs) | Done |
| Agents page with persona cards | Done |
| Docker Compose single-image deployment | Done |
| Health check endpoints (`/healthz`, `/readyz`) | 2026-06-11 |
| OpenAPI 3.0 spec + Swagger UI (`/api/v1/docs`) | 2026-06-11 |
| OpenTelemetry metrics + Prometheus exporter (`/metrics`) | 2026-06-11 |
| Agent Ingest API (push-based findings with fingerprint dedup) | 2026-06-11 |
| API token management (per-org, optional project scope) | 2026-06-11 |
| Bearer token auth middleware + subdomain org resolution | 2026-06-11 |
| CVE/CVSS scoring on findings | 2026-06-11 |
| Finding verification loop (agent confirms fixes) | 2026-06-11 |
| Reserved slug blacklist for org names | 2026-06-11 |
| Custom domain support in organizations | 2026-06-11 |
| SMTP email service (MailDev in dev, configurable in prod) | 2026-06-11 |
| Password reset via email (forgot/reset/change password) | 2026-06-11 |
| TOTP-based Multi-Factor Authentication (MFA) | 2026-06-11 |
| MFA recovery codes (10 one-time bcrypt-hashed codes) | 2026-06-11 |
| Pre-commit hook for Go/TypeScript linting | 2026-06-11 |
| Settings → Security tab (password change + MFA setup) | 2026-06-11 |
| Email verification (send + verify via token) | 2026-06-11 |
| Profile page (account security, email verification) | 2026-06-11 |
| Multi-device MFA (TOTP + email OTP, device CRUD) | 2026-06-11 |
| Multi-email management (add, verify, set primary, remove) | 2026-06-11 |
| Session tracking & management (JTI, list, revoke) | 2026-06-11 |
| Profile page redesign (3 tabs: Emails, Auth, Sessions) | 2026-06-11 |
| MFA login flow with device selector | 2026-06-11 |
| Recovery code regeneration (password-protected) | 2026-06-11 |
| Password change notification emails | 2026-06-11 |
| Login notification emails (browser, OS, IP, device) | 2026-06-11 |
| Rich session data (server-side UA parsing: browser, OS, device type) | 2026-06-11 |
| Valkey cache layer (session revocation caching, graceful fallback) | 2026-06-11 |
| Profile page redesign (4 tabs: Emails, Password, Auth, Sessions) | 2026-06-11 |
| Token Management UI (Settings → API Tokens tab) | 2026-06-11 |
| Projects page (`/projects` with create dialog) | 2026-06-11 |
| Finding detail enhancements (CVE, CVSS, seen_count, last_seen_at) | 2026-06-11 |
| Findings filters (verified status, project filter) | 2026-06-11 |
| Base domain auth restriction (auth only on base domain, cookie domain scoping, subdomain redirect) | 2026-06-11 |
| Org Creation UI (create org dialog, slug validation, auto-switch) | 2026-06-12 |
| Standardized API response envelope (Cloudflare-style `success`/`result`/`errors` format) | 2026-06-12 |
| Structured error codes (30+ codes across 8 categories with `errors.yaml` registry) | 2026-06-12 |
| Request trace IDs (`req_<hex>` on every request, in logs + headers + response bodies) | 2026-06-12 |
| Full handler migration (~250 call sites from legacy to new envelope/error format) | 2026-06-12 |
