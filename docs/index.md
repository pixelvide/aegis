# Aegis

**AI-powered security scanning platform** — multi-tenant SaaS with per-org data isolation.

Aegis runs AI agents that scan codebases for vulnerabilities, generate exploit PoCs, and present findings in a unified dashboard.

---

## What is Aegis?

Aegis is a self-hosted security platform that combines AI-powered code analysis with a multi-tenant architecture. Each organization's data is fully isolated at the database level using separate PostgreSQL schemas.

### Key Features

- **AI Security Scanning** — Automated vulnerability detection across codebases
- **Exploit Generation** — Proof-of-concept exploits for validated findings
- **Multi-Tenant Isolation** — Per-organization database schemas (Atlassian model)
- **Unified Dashboard** — Findings, scans, and triage in one place
- **Role-Based Access** — Owner, admin, member, and viewer roles per org

---

## Quick Start

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

See the [Deployment Guide](deployment.md) for production setup.

---

## Documentation

| Document | Description |
|---|---|
| [Architecture](architecture.md) | System design, multi-tenancy, data model, request lifecycle |
| [API Reference](api-reference.md) | Full endpoint documentation with examples |
| [Deployment](deployment.md) | Docker setup, CI/CD, operations, backup/restore |
| [Roadmap](roadmap.md) | Planned features and priorities |

---

## Technology Stack

| Layer | Technology |
|---|---|
| Frontend | React 19, Vite, TypeScript, Tailwind CSS, shadcn/ui |
| Backend | Go 1.25, net/http (stdlib), database/sql |
| Database | PostgreSQL 16 |
| Auth | bcrypt (cost 12), JWT (HS256, golang-jwt/v5) |
| CI/CD | GitHub Actions, release-please, GHCR |
| Deployment | Docker, Docker Compose |
