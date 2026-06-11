# API Reference

Base URL: `http://localhost:8080/api/v1`

## Authentication

### User Auth (Cookie)
All authenticated endpoints require the `aegis_token` cookie (set by login/register).  
Org-scoped endpoints additionally require `X-Org-ID` or `X-Org-Slug` header (or org subdomain when `AEGIS_BASE_DOMAIN` is set).

### Agent Auth (Bearer Token)
Agent Ingest API endpoints use `Authorization: Bearer aegis_xxx` tokens instead of cookies.  
Org context is resolved from subdomain or `X-Org-Slug` header.

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

## Scans 🔒🏢 (Read-Only)

Requires org context (`X-Org-ID` or `X-Org-Slug` header).

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
- Org context via subdomain (`acme.aegis.io`) or `X-Org-Slug` header

### POST `/agent/findings`

Push a finding. If a finding with the same `fingerprint` already exists, it updates `seen_count` and `last_seen_at` instead of creating a duplicate.

**Request:**
```json
{
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

## Token Management 🔒🏢

### POST `/tokens`

Generate a new API token. The plaintext token is returned **once** and never stored.

**Request:**
```json
{
  "name": "CI Pipeline Token",
  "project_id": "uuid",
  "expires_in": 90
}
```

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Display name (max 100 chars) |
| `project_id` | No | Scope to a specific project (empty = org-wide) |
| `expires_in` | No | Days until expiry (0 = never) |

**Response (201):**
```json
{
  "token": "aegis_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6",
  "info": {
    "id": "uuid",
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

### GET `/tokens`

List all tokens (prefix and metadata only, never the full token).

**Response (200):** Array of token objects

---

### DELETE `/tokens/{id}`

Revoke a token. Revoked tokens immediately stop working.

**Response:** `204 No Content`

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
- 🏢 — Requires org context (`X-Org-ID`, `X-Org-Slug`, or subdomain)
- 🔑 — Requires agent token (`Authorization: Bearer aegis_xxx`)
