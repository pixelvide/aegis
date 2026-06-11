# Architecture

## System Overview

Aegis is a multi-tenant security scanning platform. It follows the **BFF (Backend For Frontend)** pattern — the Go server acts as both the API and the static file server for the React SPA.

```
                  ┌──────────────────────┐
                  │      Browser         │
                  │   React SPA          │
                  └──────────┬───────────┘
                             │ HTTP (cookies)
                  ┌──────────▼───────────┐
                  │    Go Server         │
                  │                      │
                  │  ┌────────────────┐  │
                  │  │ Auth Middleware │  │  ← JWT cookie → User
                  │  └───────┬────────┘  │
                  │  ┌───────▼────────┐  │
                  │  │ Tenant MW      │  │  ← Subdomain/X-Org-Slug → Schema
                  │  └───────┬────────┘  │
                  │  ┌───────▼────────┐  │
                  │  │ API Handlers   │  │  ← findings, exploits, members,
                  │  │                │  │     projects, dashboard, tokens
                  │  └───────┬────────┘  │
                  └──────────┼───────────┘
                             │
┌─────────────────────────┐ │
│   AI Agent (external)   │ │
│   Bearer token auth     │ │
│   POST /agent/findings  ┬─┘
└─────────────────────────┘
                             │
                  ┌──────────▼───────────┐
                  │   PostgreSQL 16      │
                  │                      │
                  │  common schema       │  ← users, orgs, flags
                  │  org_<uuid> schemas  │  ← per-org data + tokens
                  └──────────────────────┘
```

---

## Multi-Tenant Data Model

### Common Schema

Shared across all organizations:

```sql
common.schema_migrations   -- version tracking
common.organizations       -- org registry (id, name, slug, plan, schema_version)
common.users               -- user accounts (email, password_hash)
common.org_members          -- org ↔ user mapping (role)
common.app_config          -- key/value settings
common.feature_flags       -- global toggleable features (platform-wide)
```

### Per-Org Schema (`org_<uuid>`)

Each org gets its own isolated schema:

```sql
org_xxx.findings              -- vulnerability findings
org_xxx.exploits              -- PoC exploits
org_xxx.api_tokens            -- agent authentication tokens
org_xxx.projects              -- code projects
org_xxx.scans                 -- security scans (legacy)
org_xxx.org_feature_flags     -- per-org feature toggles (planned)
org_xxx.org_schema_migrations -- per-org migration tracking (planned)
```

### Schema Naming

Schema names are derived from the org UUID:
```
org_<uuid-without-hyphens>
```

Example: Org `2be00885-13a1-48f6-b30e-86e02151cdd8` → schema `org_2be0088513a148f6b30e86e02151cdd8`

This is validated by regex `^org_[a-f0-9]{32}$` to prevent SQL injection.

---

## Request Lifecycle

```
1. Browser sends request with aegis_token cookie
   │
2. ServeHTTP: security headers + CORS
   │
3. Route matching (Go 1.22 method patterns)
   │
4. Auth Middleware:
   │  ├─ Read aegis_token cookie
   │  ├─ Validate JWT (HS256, check expiry, verify signing method)
   │  ├─ Load user from DB by ID
   │  └─ Inject user into context
   │
5. Tenant Middleware (for org-scoped routes):
   │  ├─ Read subdomain from Host header (production)
   │  ├─ Or read X-Org-ID / X-Org-Slug header (dev)
   │  ├─ Load org from DB
   │  ├─ Verify user is a member of the org
   │  ├─ Create schema-scoped Store
   │  └─ Inject org + store into context
   │
6. Handler: business logic + response
```

### Agent Ingest (Bearer Token)

```
1. Agent sends request with Authorization: Bearer aegis_xxx
   │
2. Token Auth Middleware:
   │  ├─ Extract Bearer token from header
   │  ├─ Resolve org from subdomain or X-Org-Slug
   │  ├─ Look up token by prefix in org's api_tokens table
   │  ├─ Verify token against bcrypt hash
   │  ├─ Check: not revoked, not expired
   │  ├─ Create schema-scoped Store
   │  └─ Inject org + store + token into context
   │
3. Handler: validate request, upsert finding, respond
```

---

## Auth Flow

### Registration
```
POST /api/v1/auth/register
  │
  ├─ Check feature flag: signup enabled?
  ├─ Validate email, password (8+ chars, upper+lower+digit), name
  ├─ Check email uniqueness
  ├─ Hash password (bcrypt cost 12)
  ├─ Create user in common.users
  ├─ Create default org ("Name's Org")
  ├─ Add user as org owner
  ├─ Generate JWT (24h TTL)
  └─ Set aegis_token HttpOnly cookie
```

### Login
```
POST /api/v1/auth/login
  │
  ├─ Find user by email
  ├─ Compare bcrypt hash (same error for wrong email/password)
  ├─ Generate JWT
  └─ Set aegis_token HttpOnly cookie
```

### Session Check
```
GET /api/v1/auth/me
  │
  ├─ Auth middleware validates cookie
  ├─ Load user + their orgs
  └─ Return { user, orgs }
```

---

## Data Isolation

Each org's data is fully isolated at the database level:

| Concern | Mechanism |
|---|---|
| Query isolation | Schema-qualified table names (`org_xxx.scans`) |
| Membership check | `TenantResolver` verifies user ∈ org before granting access |
| Schema provisioning | `ProvisionOrgSchema()` creates tables in a new schema |
| Data deletion | `DROP SCHEMA org_xxx CASCADE` removes everything |
| Schema naming | UUID-derived, regex-validated (prevents SQL injection) |

---

## Technology Stack

| Layer | Technology |
|---|---|
| Frontend | React 19, Vite, TypeScript, Tailwind CSS, shadcn/ui |
| Backend | Go 1.25, net/http (stdlib), database/sql |
| Database | PostgreSQL 16 |
| Auth (Users) | bcrypt (cost 12), JWT (HS256, golang-jwt/v5) |
| Auth (Agents) | Bearer tokens, bcrypt-hashed, per-org schema |
| Org Resolution | Subdomain (`acme.aegis.io`) or `X-Org-Slug` header |
| Deployment | Docker, Docker Compose |
| UI Embedding | Go `embed` package (SPA served from binary) |
