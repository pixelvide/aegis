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
- **Middleware chain (users):** `Auth(jwt) → TenantResolver(subdomain/custom domain + membership check) → Handler`
- **Password hash** is tagged `json:"-"` on the User model — never exposed in API responses.
- **Email enumeration** is prevented: login returns the same error for wrong email and wrong password.
- **Base domain restriction:** When `AEGIS_BASE_DOMAIN` is set, all public auth endpoints (register, login, logout, forgot-password, reset-password, MFA validate, verify-email) are blocked on org subdomains via `baseOnlyMiddleware`. Auth flows must happen on the base domain only. Cookies are set with `Domain=.baseDomain` for cross-subdomain sharing. The UI redirects users from subdomain auth pages to the base domain with a `?return_to=` param.

### Agent Authentication

- **Tokens:** Generated via `POST /api/v1/tokens` (user-facing). Format: `aegis_<32-hex-chars>`.
- **Storage:** bcrypt (cost 12) hash in `org_<uuid>.api_tokens`. Plaintext shown once, never stored.
- **Lookup:** Tokens are found by prefix (`aegis_` + 8 hex = 14 chars), then verified via bcrypt.
- **Scope:** Tokens are per-org, optionally scoped to a single project.
- **Middleware chain (agents):** `TokenAuth(bearer + subdomain/custom domain → org → prefix lookup → bcrypt verify) → Handler`
- **Context:** Use `middleware.AgentTokenFromContext(ctx)` to get the authenticated token.

### Feature Flags (IMPORTANT)

Aegis uses a **two-tier feature flag system**:

1. **Global flags** — `common.feature_flags`. Platform-wide kill-switches controlled by operators.
2. **Org-level flags** — `org_<uuid>.org_feature_flags`. Per-org overrides controlled by org admins (admin+ role).

A feature is enabled for an org only if **both** the global flag AND the org-level flag are enabled (logical AND). This means:
- If the global flag is disabled, the feature is off for **all** orgs regardless of their org-level setting.
- If the global flag is enabled but the org flag is disabled, the feature is off for **that org only**.

**Check global flags via:**
```go
if !s.common.IsFeatureEnabled(ctx, "signup") {
    // feature is globally disabled
}
```

**Check org-level flags via:**
```go
if !tenantStore.IsOrgFeatureEnabled(ctx, "require_mfa") {
    // feature is disabled for this org
}
```

**Combined check (preferred pattern for feature-gated behavior):**
```go
if !s.common.IsFeatureEnabled(ctx, "mfa") || !tenantStore.IsOrgFeatureEnabled(ctx, "require_mfa") {
    // MFA is not active for this org
}
```

**Current global flags:** `signup`, `invite_only`, `scan_docker_mode`, `public_api`.

**Example org-level flags:** `org_wide_tokens`, `require_mfa`, `ip_restrictions`, `email_domain_restriction`, `auto_join`, `api_access`, `scan_docker_mode`, `advanced_reports`, `webhooks`, `sso`, `custom_domain`.

**Rules:**
1. **All user-visible features that can be toggled MUST use org-level feature flags.** Do not use hardcoded booleans, environment variables, or config files for feature toggling. If a feature should be controllable per-org, it goes in `org_feature_flags`. If it's a platform-wide concern, it goes in `common.feature_flags`.
2. **New features default to disabled** in org-level flags. Seed sensible defaults in `ProvisionOrgSchema()` based on the org's plan.
3. **Feature flags control visibility, not authorization.** RBAC determines _who_ can do something; feature flags determine _whether_ the feature exists at all for that org.
4. **Never check feature flags in the UI by calling the API repeatedly.** Fetch all org flags once on page load via `GET /api/v1/org-features` and cache them in the org context.

### Org Versioning

Each org tracks a `schema_version` in `common.organizations`. This allows:
- **Independent upgrades:** Org A can be on schema v1 while Org B is on v2.
- **Graduated rollout:** Migrate enterprise orgs first, then pro, then free.
- **Feature-version coupling:** Org feature flags can have a `min_version` field — the feature is auto-disabled if the org hasn't been upgraded to the required schema version.

**Rules:**
1. When adding a new org-scoped table or column, update both `ProvisionOrgSchema()` (for new orgs) and add a per-org migration (for existing orgs).
2. Per-org migrations run via the `org_schema_migrations` table inside each org schema, not the global `common.schema_migrations`.
3. Never assume all orgs are on the same schema version. Always check `schema_version` before using version-dependent features.

---

## Code Conventions

### Go Server

- **Standard library only** for HTTP routing (Go 1.22+ `http.ServeMux` with method patterns like `GET /api/v1/...`).
- **No ORM** — raw SQL with `database/sql`. Use parameterized queries (`$1`, `$2`) always.
- **Package structure:**
  - `api/` — HTTP handlers, one file per domain (auth.go, agent.go, findings.go, etc.)
  - `auth/` — password hashing + JWT generation/validation
  - `config/` — environment-based configuration
  - `email/` — SMTP-based transactional email sending
  - `middleware/` — auth, tenant resolution, token auth, request ID
  - `models/` — domain types (shared between API and store)
  - `requestid/` — request ID context utilities
  - `store/` — data access (Store interface + PostgreSQL implementation)
- **API responses:** Use the standard Cloudflare-style envelope helpers:
  - `writeResult(w, r, status, data)` — single resource: `{"success": true, "result": {...}}`
  - `writeResultMessage(w, r, status, data, msg)` — resource + confirmation message
  - `writeMessage(w, r, status, msg)` — message-only (logout, password reset)
  - `writeList(w, r, data, resultInfo)` — paginated list with `result_info`
  - `writeApiError(w, r, errConstant)` — structured error from `errors.go` constants
  - `writeValidationErrors(w, r, ...FieldError{})` — field-level validation errors
  - **Never use `writeJSON` or `writeError` directly** — they are deprecated.
- **Error codes:** Use pre-defined `ApiError` constants from `api/errors.go`. Never pass raw strings. Use `.WithMessage()` to customize error messages and `.WithDetails()` for field-level validation. Add new error codes to `errors.yaml` (master registry) and regenerate `errors.go`.
- **Context:** Use `middleware.UserFromContext(ctx)` and `middleware.OrgFromContext(ctx)` to access the authenticated user and current org.
- **Tenant store:** Use `tenantStore(r)` helper in handlers (calls `middleware.TenantStoreFromContext`).
- **Imports:** Group as stdlib → external → internal.
- **Logging:** Use `log/slog` (Go stdlib). Never use `fmt.Printf`, `log.Printf`, or `log.Println` for application logging.
  - `slog.Info("message", "key", value)` — normal operational events
  - `slog.Warn("message", "key", value)` — degraded but recoverable situations
  - `slog.Error("message", "error", err)` — failures that need attention
  - `slog.Debug("message", "key", value)` — verbose tracing (only visible at debug level)
  - Always include structured key-value pairs, not formatted strings
  - For request-scoped logging, use `logger.FromContext(r.Context())` — this automatically injects `request_id` into all log lines

### React UI

- **Framework:** Vite + React + TypeScript
- **Component library:** shadcn/ui (in `components/ui/`)
- **Styling:** Tailwind CSS
- **API calls:** Always use the `request()` helper from `lib/api.ts` — it auto-injects `credentials: "include"` for cookie-based auth.
- **Auth state:** Use `useAuth()` hook from `lib/auth-context.tsx`.
- **Org state:** Use `useOrg()` hook from `lib/org-context.tsx`.
- **Pages:** One file per page in `pages/`. Keep pages focused on data fetching + layout.
- **Design:** Google Cloud Console inspired — left sidebar, flat card style, clean edges, no rounded corners on cards.

### Linting

Lint checks run automatically via a pre-commit hook (`.git/hooks/pre-commit`). You can also run them manually:

**Go server** — uses `go vet` (built-in) and optionally `golangci-lint`:
```bash
cd server && go vet ./...
# If golangci-lint is installed:
cd server && golangci-lint run ./...
```

**React UI** — uses ESLint (configured in `ui/eslint.config.js`):
```bash
cd ui && pnpm run lint
```

**Pre-commit hook:** The hook at `.git/hooks/pre-commit` runs linters only for the language with staged files. It:
- Runs `go vet ./...` (and `golangci-lint` if installed) for staged `.go` files
- Runs `pnpm exec eslint --max-warnings 0 .` for staged `.ts`/`.tsx` files
- Fails the commit if any linter reports errors

To install the hook after a fresh clone, copy it from the repo (or it's auto-created by the dev setup).

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

1. **Infrastructure (any domain, no auth)** — registered on the top-level mux outside the API server, bypasses all middleware. Accessible on base domain, org subdomains, custom domains, IPs — anywhere. Used by load balancers, Kubernetes probes, and monitoring systems: `/healthz`, `/readyz`, `/metrics`
2. **Public (base domain only)** — no auth required, but blocked on org subdomains when `AEGIS_BASE_DOMAIN` is set: `/api/v1/auth/register`, `/api/v1/auth/login`, `/api/v1/auth/logout`, `/api/v1/auth/forgot-password`, `/api/v1/auth/reset-password`, `/api/v1/auth/mfa/validate`, `/api/v1/auth/verify-email`
3. **Public (any domain)** — no auth, works everywhere: `/api/v1/config/auth`, `/api/v1/docs`, `/api/v1/docs/openapi.yaml`
4. **Authenticated** — JWT cookie required: `/api/v1/auth/me`, `/api/v1/orgs`, `/api/v1/config/features`
5. **Protected** — JWT + org context (subdomain/custom domain + membership verified): findings, projects, members, tokens, dashboard
6. **Agent** — Bearer token (org resolved from subdomain/custom domain, no membership check): `/api/v1/agent/*`

### Request/Response

- All request bodies are JSON. Responses use `application/json; charset=utf-8`.
- Request bodies are limited to 1MB (`http.MaxBytesReader`).
- `decodeJSON()` uses `DisallowUnknownFields()`.
- Successful creation returns `201 Created`.
- Successful deletion returns `204 No Content`.
- All responses use the **Cloudflare-style envelope** with `success`, `request_id`, and either `result` (success) or `errors` (failure).
- Every response includes a `request_id` (format: `req_<hex>`) in both the JSON body and the `X-Request-ID` response header.
- Errors return a structured `errors` array: `{"success": false, "errors": [{"type": "...", "code": "...", "ref": "E...", "message": "..."}]}`.

### Error Code Registry

Error codes are defined in `errors.yaml` (project root) and generated as Go constants in `api/errors.go` and TypeScript constants in `ui/src/lib/error-codes.gen.ts`.

| Type | Ref Range | Examples |
|---|---|---|
| `auth_error` | E10001–E10007 | `not_authenticated`, `invalid_credentials`, `mfa_required` |
| `token_error` | E20001–E20004 | `invalid`, `expired`, `scope_mismatch` |
| `tenant_error` | E30001–E30005 | `not_found`, `not_member`, `slug_taken` |
| `resource_error` | E40001–E40003 | `not_found`, `conflict`, `already_exists` |
| `validation_error` | E50001–E50004 | `invalid_request`, `field_required`, `field_invalid` |
| `permission_error` | E60001–E60004 | `denied`, `feature_disabled`, `mfa_required_by_org`, `feature_not_provisioned` |
| `rate_limit_error` | E70001 | `exceeded` |
| `server_error` | E90001–E90002 | `internal`, `unavailable` |

**Rules:**
1. Every error response MUST use a pre-defined `ApiError` constant from `errors.go` — never pass raw strings.
2. Error messages are human-readable and MAY change — the `code` and `ref` fields are the stable contract.
3. New error scenarios MUST define a new code in `errors.yaml` — never reuse an existing code for a different meaning.
4. Use `.WithMessage()` to customize error messages for specific contexts (e.g., `errValidationFieldInvalid.WithMessage("invalid email")`).
5. Use `.WithDetails()` or `writeValidationErrors()` for field-level validation errors.

### Request IDs

- The `RequestID` middleware generates a unique `req_<hex>` ID for every request.
- It is the **outermost middleware** — runs before auth, CORS, or any handler.
- Request IDs are injected into the request context and available via `requestid.FromContext(ctx)`.
- All `writeResult`/`writeApiError` helpers automatically include the request ID.
- `logger.FromContext(r.Context())` automatically adds `request_id` to all log output.
- Incoming `X-Request-ID` headers from reverse proxies are accepted and preserved.

### Org Context

Org-scoped requests are resolved from the request's `Host` header:
- **Subdomain**: `acme.aegis.io` → slug `acme`
- **Custom domain**: `security.acme.com` → looked up in `common.organizations.custom_domain`

`AEGIS_BASE_DOMAIN` defaults to `lvh.me` for local development (`*.lvh.me` resolves to `127.0.0.1`).

The `TenantResolver` middleware (for users) or `TokenAuth` middleware (for agents) resolves this to an org, creates a schema-scoped store, and injects both into the request context.

### Reserved Slugs

Org slugs are validated against a blacklist of reserved names (`admin`, `api`, `dashboard`, `www`, etc.) in `store.IsReservedSlug()`. This prevents subdomain conflicts.

---

## Development Environment (IMPORTANT)

### Docker-First Philosophy

**Always prefer Docker** for running services and building the project. If Docker and Docker Compose are available on the machine, use them for everything possible — database, SMTP, builds, and the app itself.

**Decision flow:**
1. **Check if Docker is installed:** Run `docker --version` and `docker compose version`.
2. **If Docker is available:** Use `docker compose up --build -d` to start everything. No other prerequisites needed.
3. **If Docker is NOT available:** Fall back to local installation (see Prerequisites below). **Ask the USER** before installing anything — do not silently install tools.

### Prerequisites (Local Development — No Docker)

If Docker is unavailable, the following must be installed locally. **Before proceeding, ask the USER to install any missing tools** or get their permission to install them.

| Tool | Minimum Version | Purpose | Install Check |
|---|---|---|---|
| Go | 1.25+ | Server compilation | `go version` |
| Node.js | 24+ | UI build & dev server | `node --version` |
| pnpm | 9+ | UI dependency management | `pnpm --version` |
| PostgreSQL | 16+ | Database | `psql --version` |
| Git | 2.0+ | Version control | `git --version` |

**Rules:**
1. **Never assume tools are installed.** Always check first with the version commands above.
2. **If a prerequisite is missing**, inform the USER which tool is needed and why, then ask them to install it. Do not attempt to install system packages (e.g., via `apt`, `brew`, `yum`) without explicit USER approval.
3. **For pnpm dependencies**, run `cd ui && pnpm install` only after confirming Node.js and pnpm are available.
4. **For Go modules**, run `cd server && go mod download` only after confirming Go is available.

### Running Locally (Without Docker)

If running without Docker, the USER needs to manually set up:

1. **PostgreSQL** — Running locally or remotely, with a database created:
   ```bash
   createdb aegis
   ```
2. **Environment variables** — Copy `.env.example` to `.env` and configure `DATABASE_URL`, `JWT_SECRET`, etc.
3. **Server:**
   ```bash
   cd server && go run ./cmd/server
   ```
4. **UI (dev mode):**
   ```bash
   cd ui && pnpm install && pnpm run dev
   ```

---

## Docker

### Production Build

```bash
docker compose up --build -d
```

The `server/Dockerfile` is a 3-stage build:
1. **node:24-alpine** — builds the React UI (`pnpm run build`)
2. **golang:1.25-alpine** — compiles the Go server with the UI embedded
3. **alpine:3.20** — minimal runtime (~15MB)

### Port Mapping

| Service | Internal | External (default) |
|---|---|---|
| PostgreSQL | 5432 | 5432 |
| Valkey | 6379 | 6379 |
| Aegis Server | 8080 | 8080 |
| MailDev SMTP | 1025 | 1025 |
| MailDev Web UI | 1080 | 1080 |

In production, the Go server serves both the API (`/api/*`) and the SPA UI (`/`) on port 8080.

### SMTP / Email

Transactional emails (password reset, MFA codes, etc.) are sent via SMTP. Configuration is environment-based:

| Variable | Default | Description |
|---|---|---|
| `SMTP_HOST` | `localhost` | SMTP server hostname |
| `SMTP_PORT` | `1025` | SMTP server port |
| `SMTP_USERNAME` | *(empty)* | Auth username (empty = no auth) |
| `SMTP_PASSWORD` | *(empty)* | Auth password |
| `SMTP_FROM` | `noreply@aegis.local` | Sender address |
| `SMTP_TLS` | `false` | Enable STARTTLS (`true` for production) |

**Development:** MailDev runs automatically via Docker Compose. View sent emails at `http://localhost:1080`.

**Production:** Set real SMTP credentials (SendGrid, AWS SES, Mailgun, etc.) in `.env`.

### Logging

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `info` | Minimum log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `text` | Output format: `text` (human-readable) or `json` (structured for log aggregation) |

Uses Go's stdlib `log/slog`. JSON format is recommended for production with log aggregation (ELK, Loki, CloudWatch, etc.).

---

## Testing

### Manual API Tests

```bash
# Register
curl -c cookies.txt -X POST http://lvh.me:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"SecureP@ss1","name":"Test"}'
# Response: {"success": true, "request_id": "req_...", "result": {"user": {...}}, "message": "registration successful"}

# Login
curl -c cookies.txt -X POST http://lvh.me:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"SecureP@ss1"}'

# Authenticated request
curl -b cookies.txt http://lvh.me:8080/api/v1/auth/me

# Org-scoped request (use org slug from /api/v1/orgs response)
curl -b cookies.txt http://<org-slug>.lvh.me:8080/api/v1/findings

# Create an API token
curl -b cookies.txt -X POST http://<org-slug>.lvh.me:8080/api/v1/tokens \
  -H "Content-Type: application/json" \
  -d '{"name":"CI Token","expires_in":90}'
# Save the "result.token" field from the response!

# Agent: push a finding (Bearer token)
curl -X POST http://<org-slug>.lvh.me:8080/api/v1/agent/findings \
  -H "Authorization: Bearer aegis_a1b2c3d4..." \
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
