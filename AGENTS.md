# AGENTS.md — Coding Guidelines for Aegis

This file provides context and rules for AI agents working on the Aegis codebase.

---

## Project Overview

Aegis is a multi-tenant AI security scanning platform. It has two main components:

- **Go server** (`server/`) — API, auth, agent ingest, multi-tenant store
- **React UI** (`ui/`) — Vite + shadcn/ui + Tailwind

They are deployed as a single Docker image (UI embedded into the Go binary via `//go:embed`).

---

## Architecture Rules

### Multi-Tenancy (CRITICAL)

Aegis uses a **multi-schema PostgreSQL architecture** (Atlassian model):

- `common` schema — users, organizations, org_members, app_config, feature_flags, schema_migrations
- `org_<uuid>` schema — per-org data (findings, exploits, api_tokens, projects, scans)

**Rules:**
1. **Never query across org schemas.** Each tenant's data lives in its own schema.
2. Schema names follow the pattern `org_<32-char-hex>` (UUID without hyphens). Validated by regex `^org_[a-f0-9]{32}$`.
3. All org-scoped queries go through `store.Postgres` (the tenant store), which uses `schema.table` qualified names via the `p.t("table")` helper.
4. Cross-org data (users, orgs, memberships) goes through `store.CommonStore`.
5. The `TenantResolver` middleware injects the correct schema-scoped store into the request context.

### Schema Migrations

Migrations are versioned and tracked in `common.schema_migrations`.

**To add a new migration:**
1. Open `server/internal/store/common.go`
2. Append to the `migrations` slice (never edit existing entries)
3. Use the next sequential version number
4. Each migration runs in a transaction — it either fully applies or rolls back

```go
{
    Version:     5,
    Description: "Add description column to projects",
    SQL:         `ALTER TABLE common.some_table ADD COLUMN x TEXT DEFAULT '';`,
},
```

**For tenant schema changes**, update `ProvisionOrgSchema()` in `common.go` — new orgs get the latest schema. For existing orgs, add an `ALTER TABLE` migration that iterates schemas (or handle it at the application level).

### Authentication

- **Passwords:** bcrypt cost 12, via `auth.HashPassword()` / `auth.CheckPassword()`
- **Sessions:** JWT stored in `aegis_token` HttpOnly cookie (SameSite=Lax, 24h TTL)
- **Token signing:** HS256 with `JWT_SECRET` env var. Auto-generated in dev (ephemeral).
- **Middleware chain (users):** `Auth(jwt) → TenantResolver(subdomain/header + membership check) → Handler`
- **Password hash** is tagged `json:"-"` on the User model — never exposed in API responses.
- **Email enumeration** is prevented: login returns the same error for wrong email and wrong password.

### Agent Authentication

- **Tokens:** Generated via `POST /api/v1/tokens` (user-facing). Format: `aegis_<32-hex-chars>`.
- **Storage:** bcrypt (cost 12) hash in `org_<uuid>.api_tokens`. Plaintext shown once, never stored.
- **Lookup:** Tokens are found by prefix (`aegis_` + 8 hex = 14 chars), then verified via bcrypt.
- **Scope:** Tokens are per-org, optionally scoped to a single project.
- **Middleware chain (agents):** `TokenAuth(bearer + subdomain/header → org → prefix lookup → bcrypt verify) → Handler`
- **Context:** Use `middleware.AgentTokenFromContext(ctx)` to get the authenticated token.

### Feature Flags

Feature flags live in `common.feature_flags`. Check them via:
```go
if !s.common.IsFeatureEnabled(ctx, "signup") {
    // feature is disabled
}
```

Current flags: `signup`, `invite_only`, `scan_docker_mode`, `public_api`.

---

## Code Conventions

### Go Server

- **Standard library only** for HTTP routing (Go 1.22+ `http.ServeMux` with method patterns like `GET /api/v1/...`).
- **No ORM** — raw SQL with `database/sql`. Use parameterized queries (`$1`, `$2`) always.
- **Package structure:**
  - `api/` — HTTP handlers, one file per domain (auth.go, agent.go, findings.go, etc.)
  - `auth/` — password hashing + JWT generation/validation
  - `config/` — environment-based configuration
  - `middleware/` — auth, tenant resolution, and token auth
  - `models/` — domain types (shared between API and store)
  - `store/` — data access (Store interface + PostgreSQL implementation)
- **Error responses:** Use `writeError(w, status, "message")` — returns `{"error": "message"}`.
- **Context:** Use `middleware.UserFromContext(ctx)` and `middleware.OrgFromContext(ctx)` to access the authenticated user and current org.
- **Tenant store:** Use `tenantStore(r)` helper in handlers (calls `middleware.TenantStoreFromContext`).
- **Imports:** Group as stdlib → external → internal.

### React UI

- **Framework:** Vite + React + TypeScript
- **Component library:** shadcn/ui (in `components/ui/`)
- **Styling:** Tailwind CSS
- **API calls:** Always use the `request()` helper from `lib/api.ts` — it auto-injects `X-Org-ID` and `credentials: "include"`.
- **Auth state:** Use `useAuth()` hook from `lib/auth-context.tsx`.
- **Org state:** Use `useOrg()` hook from `lib/org-context.tsx`.
- **Pages:** One file per page in `pages/`. Keep pages focused on data fetching + layout.
- **Design:** Google Cloud Console inspired — left sidebar, flat card style, clean edges, no rounded corners on cards.

### Git Conventions

This project uses **[Conventional Commits](https://www.conventionalcommits.org/)** for all commit messages. This is required — `release-please` parses commit messages to auto-generate changelogs and determine version bumps.

**Format:** `<type>(<scope>): <description>`

| Type | When to Use | Version Bump |
|---|---|---|
| `feat` | New feature or capability | Minor |
| `fix` | Bug fix | Patch |
| `docs` | Documentation only | None |
| `refactor` | Code change that doesn't fix a bug or add a feature | None |
| `perf` | Performance improvement | Patch |
| `test` | Adding or fixing tests | None |
| `ci` | CI/CD changes (workflows, configs) | None |
| `chore` | Maintenance (deps, tooling, etc.) | None |

**Scope** is optional but encouraged — use the component name: `api`, `ui`, `store`, `auth`, `docker`, `docs`.

**Breaking changes:** Add `!` after the type or include `BREAKING CHANGE:` in the commit footer. This triggers a major version bump.

**Examples:**
```
feat(api): add member invitation endpoint
fix(store): prevent duplicate org schema creation
docs: update API reference with new scan fields
feat(ui)!: redesign dashboard layout
ci: add Docker build verification to CI
```

---

## API Design

### Route Tiers

1. **Public** — no auth required: `/api/v1/auth/register`, `/api/v1/auth/login`, `/api/v1/auth/logout`
2. **Authenticated** — JWT cookie required: `/api/v1/auth/me`, `/api/v1/orgs`, `/api/v1/config/features`
3. **Protected** — JWT + org context (subdomain/X-Org-Slug + membership verified): findings, projects, members, tokens, dashboard
4. **Agent** — Bearer token (org resolved from subdomain/header, no membership check): `/api/v1/agent/*`

### Request/Response

- All request bodies are JSON. Responses use `application/json; charset=utf-8`.
- Request bodies are limited to 1MB (`http.MaxBytesReader`).
- `decodeJSON()` uses `DisallowUnknownFields()`.
- Successful creation returns `201 Created`.
- Successful deletion returns `204 No Content`.
- Errors return `{"error": "human-readable message"}`.

### Org Context

Org-scoped requests must include one of:
- **Subdomain** (production): `acme.aegis.io` → slug `acme` (when `AEGIS_BASE_DOMAIN` is set)
- **Custom domain**: `security.acme.com` → looked up in `common.organizations.custom_domain`
- `X-Org-ID: <uuid>` header
- `X-Org-Slug: <slug>` header

The `TenantResolver` middleware (for users) or `TokenAuth` middleware (for agents) resolves this to an org, creates a schema-scoped store, and injects both into the request context.

### Reserved Slugs

Org slugs are validated against a blacklist of reserved names (`admin`, `api`, `dashboard`, `www`, etc.) in `store.IsReservedSlug()`. This prevents subdomain conflicts.

---

## Docker

### Production Build

```bash
docker compose up --build -d
```

The `server/Dockerfile` is a 3-stage build:
1. **node:20-alpine** — builds the React UI (`npm run build`)
2. **golang:1.25-alpine** — compiles the Go server with the UI embedded
3. **alpine:3.20** — minimal runtime (~15MB)

### Port Mapping

| Service | Internal | External (default) |
|---|---|---|
| PostgreSQL | 5432 | 5432 |
| Aegis Server | 8080 | 8080 |

In production, the Go server serves both the API (`/api/*`) and the SPA UI (`/`) on port 8080.

---

## Testing

### Manual API Tests

```bash
# Register
curl -c cookies.txt -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"SecureP@ss1","name":"Test"}'

# Login
curl -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"SecureP@ss1"}'

# Authenticated request
curl -b cookies.txt http://localhost:8080/api/v1/auth/me

# Org-scoped request
curl -b cookies.txt http://localhost:8080/api/v1/findings -H "X-Org-Slug: test"

# Create an API token
curl -b cookies.txt -X POST http://localhost:8080/api/v1/tokens \
  -H "Content-Type: application/json" \
  -H "X-Org-Slug: test" \
  -d '{"name":"CI Token","expires_in":90}'
# Save the "token" field from the response!

# Agent: push a finding (Bearer token)
curl -X POST http://localhost:8080/api/v1/agent/findings \
  -H "Authorization: Bearer aegis_a1b2c3d4..." \
  -H "X-Org-Slug: test" \
  -H "Content-Type: application/json" \
  -d '{"project_id":"uuid","fingerprint":"sha256:abc","title":"XSS","severity":"high","description":"..."}'
```

---

## Common Pitfalls

1. **Port 8080 in use** — Kill existing process: `lsof -ti :8080 | xargs kill -9`
2. **CREATE TABLE IF NOT EXISTS won't add columns** — Use the migration system.
3. **JWT_SECRET not set** — In dev, an ephemeral secret is auto-generated (sessions don't survive restart). Always set `JWT_SECRET` in `.env` for production.
4. **CORS cookies** — The frontend must use `credentials: "include"` and the server must return `Access-Control-Allow-Credentials: true`.
5. **Schema naming** — Always use `org.SchemaName()` to get the schema name. Never construct it manually.

---

## Documentation (IMPORTANT)

After completing any work, you **must** update the relevant documentation to reflect your changes. Outdated docs are treated as bugs.

### What to Update

| Change Type | Docs to Update |
|---|---|
| New or modified API endpoints | `docs/api-reference.md` |
| Architectural changes (new packages, schema changes, new middleware) | `docs/architecture.md` |
| New environment variables, Docker changes, or infra updates | `docs/deployment.md` |
| New features or completed roadmap items | `docs/roadmap.md` |
| Changes to project setup, build steps, or high-level overview | `README.md` |
| Changes to coding conventions, patterns, or project rules | `AGENTS.md` |

### Rules

1. **Always check** if your changes affect any of the docs listed above before finishing.
2. **Add** new sections or entries for new features, endpoints, or config options.
3. **Update** existing sections when behavior, defaults, or interfaces change.
4. **Remove** outdated information that no longer applies (deprecated endpoints, removed flags, etc.).
5. **Keep the style consistent** with the existing documentation — match the formatting, heading levels, and tone already in use.
6. Documentation updates should be part of the **same work session**, not deferred to a follow-up.
