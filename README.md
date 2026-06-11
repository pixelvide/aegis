# Aegis

[![CI](https://github.com/pixelvide/aegis/actions/workflows/ci.yml/badge.svg)](https://github.com/pixelvide/aegis/actions/workflows/ci.yml)
[![Release](https://github.com/pixelvide/aegis/actions/workflows/release-please.yml/badge.svg)](https://github.com/pixelvide/aegis/actions/workflows/release-please.yml)
[![Docs](https://github.com/pixelvide/aegis/actions/workflows/docs-pages.yml/badge.svg)](https://pixelvide.github.io/aegis/)

**AI-powered security scanning platform** — multi-tenant SaaS with per-org data isolation.

Aegis provides a centralized platform where external AI security agents push vulnerability findings via authenticated REST API. Findings are deduplicated, triaged, and presented in a unified dashboard.

---

## Quick Start

### Docker Compose (recommended)

```bash
# Clone and start
git clone git@github.com:pixelvide/aegis.git
cd aegis

# Create env file
cp .env.example .env
# Edit .env — set JWT_SECRET for production

# Start everything
docker compose up --build -d

# Open http://localhost:8080
```

### Local Development

```bash
# 1. Start PostgreSQL
docker compose up postgres -d

# 2. Start Go server
cd server
export DATABASE_URL="postgres://aegis:aegis@localhost:5432/aegis?sslmode=disable"
go run ./cmd/aegis-server/

# 3. Start UI dev server (separate terminal)
cd ui
npm install
npm run dev
# Open http://localhost:5173
```

### Subdomain Testing (local)

Aegis supports subdomain-based org resolution (e.g., `acme.aegis.io`). To test this locally, we use [`lvh.me`](http://lvh.me) — a public domain where `*.lvh.me` resolves to `127.0.0.1`. No `/etc/hosts` changes needed.

**Setup:**

```bash
# .env (already set by default in .env.example)
AEGIS_BASE_DOMAIN=lvh.me

# Restart
docker compose up --build -d
```

**Usage:**

| URL | What happens |
|---|---|
| `http://lvh.me:8080` | Base domain — no subdomain, header-only mode |
| `http://test.lvh.me:8080` | Resolves org with slug `test` from subdomain |
| `http://acme.lvh.me:8080` | Resolves org with slug `acme` from subdomain |
| Any new org slug | Works instantly — `*.lvh.me` is a wildcard |

**Testing the mismatch guard:**

```bash
# This works — subdomain matches
curl -b cookies.txt http://test.lvh.me:8080/api/v1/findings

# This is REJECTED (400) — subdomain says "test" but header says "acme"
curl -b cookies.txt http://test.lvh.me:8080/api/v1/findings \
  -H "X-Org-Slug: acme"
# → {"error":"X-Org-Slug header conflicts with subdomain"}
```

> **Production:** Set `AEGIS_BASE_DOMAIN=aegis.io` (or your domain). Configure DNS with a wildcard `*.aegis.io → your-server-ip`.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Browser (:5173 dev / :8080 prod)         │
│  React + shadcn/ui + Tailwind                                   │
│  Login → Dashboard → Scans → Findings → Settings                │
└──────────────────────────┬──────────────────────────────────────┘
                           │ /api/v1/*
┌──────────────────────────▼──────────────────────────────────────┐
│                        Go Server (:8080)                        │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │ Auth MW  │→ │ Tenant MW    │→ │ Handlers                 │  │
│  │ (JWT     │  │ (subdomain / │  │ findings, exploits,      │  │
│  │  cookie) │  │  X-Org-Slug) │  │ orgs, projects, members  │  │
│  └──────────┘  └──────────────┘  └──────────────────────────┘  │
│  ┌──────────┐  ┌──────────────────────────────────────────────┐ │
│  │ Token MW │→ │ Agent Ingest API (Bearer token auth)         │ │
│  │ (Bearer) │  │ POST/GET/PATCH /api/v1/agent/findings        │ │
│  └──────────┘  └──────────────────────────────────────────────┘ │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                     PostgreSQL 16                                │
│  ┌─────────────┐  ┌──────────────────┐  ┌────────────────────┐ │
│  │ common      │  │ org_<uuid_1>     │  │ org_<uuid_2>       │ │
│  │ users       │  │ findings         │  │ findings           │ │
│  │ orgs        │  │ exploits         │  │ exploits           │ │
│  │ org_members │  │ api_tokens       │  │ api_tokens         │ │
│  │ migrations  │  │ projects         │  │ projects           │ │
│  │ flags       │  │ scans (legacy)   │  │ scans (legacy)     │ │
│  └─────────────┘  └──────────────────┘  └────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Multi-Tenancy (Atlassian Model)

Each organization gets its own PostgreSQL schema (`org_<uuid>`). The `common` schema holds shared data (users, organizations, memberships). This provides:

- **Full data isolation** — one org can never see another's data
- **Independent schema evolution** — per-org migrations possible
- **Easy data export/deletion** — `DROP SCHEMA org_xxx CASCADE`

---

## Project Structure

```
aegis/
├── docker-compose.yml          # PostgreSQL + Aegis server
├── .env.example                # Configuration template
├── AGENTS.md                   # AI coding agent guidelines
├── server/
│   ├── Dockerfile              # Multi-stage build (UI + Go)
│   ├── go.mod
│   ├── cmd/aegis-server/
│   │   ├── main.go             # Entry point
│   │   └── ui.go               # Embedded UI handler (SPA)
│   └── internal/
│       ├── api/                # HTTP handlers
│       │   ├── router.go       # Routes + middleware wiring
│       │   ├── auth.go         # Register, login, logout, me
│       │   ├── agent.go        # Agent ingest + token management
│       │   ├── scans.go        # Scan list/get (read-only, legacy)
│       │   ├── findings.go     # Finding CRUD + triage
│       │   ├── orgs.go         # Organization CRUD
│       │   ├── projects.go     # Project CRUD
│       │   ├── members.go      # Member invite/remove
│       │   └── dashboard.go    # Dashboard stats
│       ├── auth/               # JWT + bcrypt service
│       ├── config/             # Env-based configuration
│       ├── email/
│       │   ├── email.go        # SMTP transport service
│       │   └── templates/      # HTML email templates
│       │       ├── layout.go           # Shared base layout + helpers
│       │       ├── login_alert.go      # New sign-in notification
│       │       ├── password_reset.go   # Reset link email
│       │       ├── password_changed.go # Change confirmation
│       │       ├── verify_email.go     # Email verification link
│       │       └── mfa_code.go         # MFA OTP code
│       ├── middleware/
│       │   ├── auth.go         # JWT cookie validation
│       │   ├── tenant.go       # Org resolution + membership check
│       │   └── token.go        # Bearer token auth for agents
│       ├── models/             # Domain types
│       │   ├── scan.go         # Scan, Target, Summary
│       │   ├── finding.go      # Finding, Exploit
│       │   ├── token.go        # APIToken
│       │   └── tenant.go       # Organization, User, OrgMember, Project
│       └── store/
│           ├── store.go        # Store interface
│           ├── common.go       # Common schema + migrations
│           └── postgres.go     # Tenant-scoped queries
├── ui/
│   ├── package.json
│   └── src/
│       ├── App.tsx             # Auth + routing
│       ├── lib/
│       │   ├── api.ts          # API client (auto org headers)
│       │   ├── auth-context.tsx # Auth state + redirect
│       │   ├── org-context.tsx  # Org selection + persistence
│       │   └── types.ts        # TypeScript domain types
│       ├── pages/
│       │   ├── login.tsx        # Auth page
│       │   ├── dashboard.tsx    # Stats + metric cards
│       │   ├── scans.tsx        # Scan list table
│       │   ├── findings.tsx     # Findings list with filters
│       │   ├── finding-detail.tsx # Full finding + exploits
│       │   ├── agents.tsx       # Agent persona cards
│       │   └── settings.tsx     # General + Members tabs
│       └── components/
│           ├── app-sidebar.tsx  # Left nav + user menu
│           ├── top-nav.tsx      # Breadcrumb header
│           ├── org-switcher.tsx
│           ├── metric-card.tsx  # Dashboard stat cards
│           ├── severity-badge.tsx
│           └── ui/             # shadcn/ui primitives
└── docs/
    ├── architecture.md          # System design + data model
    ├── api-reference.md         # Full API documentation
    ├── deployment.md            # Docker + ops guide
    └── roadmap.md               # Feature roadmap
```

---

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | *(required)* | PostgreSQL connection string |
| `JWT_SECRET` | *(auto-gen dev)* | Secret for JWT signing. **Set in production.** |
| `AEGIS_PORT` | `8080` | HTTP listen port |
| `AEGIS_BIND` | `127.0.0.1` | Bind address (`0.0.0.0` in Docker) |
| `AEGIS_BASE_DOMAIN` | *(empty)* | Base domain for subdomain org resolution (e.g., `aegis.io`) |
| `AEGIS_ALLOWED_ORIGINS` | `http://localhost:5173` | CORS origins (comma-separated) |
| `POSTGRES_USER` | `aegis` | PostgreSQL user |
| `POSTGRES_PASSWORD` | `aegis` | PostgreSQL password |
| `POSTGRES_DB` | `aegis` | PostgreSQL database name |

---

## API Overview

### Auth (public)
| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/v1/auth/register` | Create account (gated by `signup` flag) |
| POST | `/api/v1/auth/login` | Sign in → sets `aegis_token` cookie |
| POST | `/api/v1/auth/logout` | Clear auth cookie |
| GET | `/api/v1/auth/me` | Current user + their orgs |

### Organizations (authenticated)
| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/v1/orgs` | Create org (creator = owner) |
| GET | `/api/v1/orgs` | List user's orgs + `base_domain` config |
| GET | `/api/v1/orgs/{slug}` | Get org by slug |

### Feature Flags (authenticated)
| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/v1/config/features` | List all feature flags |

### Scans (authenticated + org context) — read-only
| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/v1/scans` | List scans (legacy data) |
| GET | `/api/v1/scans/{id}` | Get scan details |

### Findings (authenticated + org context)
| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/v1/findings` | List findings (filterable) |
| GET | `/api/v1/findings/{id}` | Get finding details |
| PATCH | `/api/v1/findings/{id}` | Update finding status |
| GET | `/api/v1/findings/{id}/exploits` | List exploits |
| GET | `/api/v1/findings/{id}/exploits/{eid}` | Get specific exploit |

### Dashboard (authenticated + org context)
| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/v1/dashboard/stats` | Aggregate statistics |

### Projects & Members (authenticated + org context)
| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/v1/projects` | Create project |
| GET | `/api/v1/projects` | List org projects |
| GET | `/api/v1/projects/{slug}` | Get project by slug |
| GET | `/api/v1/members` | List org members |
| POST | `/api/v1/members/invite` | Invite user by email |
| DELETE | `/api/v1/members/{userId}` | Remove member |

### Agent Ingest API (Bearer token auth)
| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/v1/agent/findings` | Push a finding (upsert on fingerprint) |
| GET | `/api/v1/agent/findings` | Pull findings for verification |
| PATCH | `/api/v1/agent/findings/{id}` | Update finding status (open/verified) |
| POST | `/api/v1/agent/findings/{id}/exploits` | Attach exploit code |

### Token Management (authenticated + org context)
| Method | Endpoint | Description |
|---|---|---|
| POST | `/api/v1/tokens` | Generate API token (shown once) |
| GET | `/api/v1/tokens` | List tokens (prefix + metadata) |
| DELETE | `/api/v1/tokens/{id}` | Revoke token |

Org-scoped endpoints require an `X-Org-ID` or `X-Org-Slug` header (or subdomain when `AEGIS_BASE_DOMAIN` is set).

Agent endpoints use `Authorization: Bearer aegis_xxx` tokens instead of cookies.

### Operational (public)
| Method | Endpoint | Description |
|---|---|---|
| GET | `/healthz` | Liveness probe (checks DB connectivity) |
| GET | `/readyz` | Readiness probe |
| GET | `/metrics` | Prometheus metrics |
| GET | `/api/v1/docs` | Swagger UI |
| GET | `/api/v1/docs/openapi.yaml` | OpenAPI 3.0 spec |

> See [docs/api-reference.md](docs/api-reference.md) for full request/response examples.

---

## Documentation

📖 **[View the full documentation site →](https://pixelvide.github.io/aegis/)**

| Document | Description |
|---|---|
| [Architecture](docs/architecture.md) | System design, multi-tenancy, data model, request lifecycle |
| [API Reference](docs/api-reference.md) | Full endpoint documentation with examples |
| [Deployment](docs/deployment.md) | Docker setup, CI/CD, operations, backup/restore |
| [Roadmap](docs/roadmap.md) | Planned features and priorities |

---

## License

Proprietary — © Pixelvide
