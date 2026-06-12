# Deployment Guide

## Docker Compose (Single Server)

The simplest deployment. Runs PostgreSQL + Aegis on a single machine.

### 1. Prepare

```bash
cd aegis

# Create env file from template
cp .env.example .env
```

### 2. Configure `.env`

```bash
# REQUIRED for production
JWT_SECRET=your-secure-random-string-here-at-least-32-chars

# PostgreSQL credentials (change from defaults)
POSTGRES_USER=aegis
POSTGRES_PASSWORD=a-strong-password-here
POSTGRES_DB=aegis

# Server
AEGIS_PORT=8080
AEGIS_ALLOWED_ORIGINS=https://your-domain.com

# Subdomain-based org resolution
# Development: lvh.me (*.lvh.me resolves to 127.0.0.1)
# Production: your-domain.com (configure wildcard DNS: *.your-domain.com)
AEGIS_BASE_DOMAIN=lvh.me      # dev
# AEGIS_BASE_DOMAIN=aegis.io  # production

# Logging (default: info level, text format)
# Use json format for production log aggregation (ELK, Loki, etc.)
LOG_LEVEL=info
LOG_FORMAT=text
```

Generate a strong JWT secret:
```bash
openssl rand -hex 32
```

### 3. Build & Start

```bash
docker compose up --build -d
```

### 4. Verify

```bash
# Check services
docker compose ps

# Check logs
docker compose logs -f aegis-server

# Health check
curl -s http://localhost:8080/healthz | jq .
# Should return {"status":"ok","db":"ok","timestamp":"..."}

# Prometheus metrics
curl -s http://localhost:8080/metrics | head -20

# API docs
# Open http://localhost:8080/api/v1/docs in browser
```

### 5. Reverse Proxy (Production)

Use nginx or Caddy in front of Aegis for TLS:

**Caddy (automatic HTTPS):**
```
aegis.example.com {
    reverse_proxy localhost:8080
}
```

**nginx:**
```nginx
server {
    listen 443 ssl;
    server_name aegis.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

> **Important:** When behind a reverse proxy with HTTPS, update the auth cookie to use `Secure: true` in `api/auth.go`.

---

## Operations

### Backup

```bash
# Dump entire database
docker exec aegis-postgres pg_dump -U aegis aegis > backup.sql

# Dump specific org schema
docker exec aegis-postgres pg_dump -U aegis -n org_<uuid> aegis > org_backup.sql
```

### Restore

```bash
docker exec -i aegis-postgres psql -U aegis aegis < backup.sql
```

### View Migrations

```bash
docker exec aegis-postgres psql -U aegis -c \
  "SELECT version, description, applied_at FROM common.schema_migrations ORDER BY version;"
```

### View Feature Flags

```bash
docker exec aegis-postgres psql -U aegis -c \
  "SELECT name, enabled, description FROM common.feature_flags;"
```

### Toggle Feature Flag

```bash
# Disable signup
docker exec aegis-postgres psql -U aegis -c \
  "UPDATE common.feature_flags SET enabled = FALSE WHERE name = 'signup';"

# Enable invite-only mode
docker exec aegis-postgres psql -U aegis -c \
  "UPDATE common.feature_flags SET enabled = TRUE WHERE name = 'invite_only';"
```

### List Orgs & Schemas

```bash
docker exec aegis-postgres psql -U aegis -c \
  "SELECT id, name, slug, plan FROM common.organizations;"

# List all org schemas
docker exec aegis-postgres psql -U aegis -c \
  "SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'org_%';"
```

### Delete an Org (nuclear)

```bash
# Get the org's schema name first
docker exec aegis-postgres psql -U aegis -c \
  "SELECT id, name, slug FROM common.organizations WHERE slug = 'org-slug';"

# Drop the schema (removes ALL org data)
docker exec aegis-postgres psql -U aegis -c \
  "DROP SCHEMA IF EXISTS org_<uuid> CASCADE;"

# Remove the org record
docker exec aegis-postgres psql -U aegis -c \
  "DELETE FROM common.organizations WHERE slug = 'org-slug';"
```

---

## Updating

```bash
# Pull latest code
git pull

# Rebuild and restart
docker compose up --build -d

# Migrations run automatically on startup
docker compose logs aegis-server | grep "migration"
```

---

## Monitoring

### Health Checks

Aegis exposes standard health probe endpoints:

| Endpoint | Purpose |
|---|---|
| `GET /healthz` | Liveness probe — checks DB connectivity |
| `GET /readyz` | Readiness probe — same as healthz |

The Docker Compose healthcheck uses `/healthz` automatically.

### Prometheus Metrics

Aegis exposes a `/metrics` endpoint in Prometheus text format. To scrape it, add this to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: aegis
    metrics_path: /metrics
    static_configs:
      - targets: ["localhost:8080"]
```

### Swagger / API Docs

The interactive API documentation is available at `http://lvh.me:8080/api/v1/docs`.

The raw OpenAPI 3.0 spec is at `http://lvh.me:8080/api/v1/docs/openapi.yaml`.

---

## Troubleshooting

| Problem | Solution |
|---|---|
| Port 8080 in use | `lsof -ti :8080 \| xargs kill -9` |
| DB connection refused | Check `docker compose ps` — is postgres healthy? |
| JWT errors after restart | Set `JWT_SECRET` in `.env` (ephemeral secret changes on restart) |
| CORS errors | When `AEGIS_BASE_DOMAIN` is set, subdomain origins are auto-allowed. Otherwise, add your domain to `AEGIS_ALLOWED_ORIGINS` |
| Cookies not sent | Ensure `credentials: "include"` in fetch and `Access-Control-Allow-Credentials` on server |
| Migration failed | Check `docker compose logs aegis-server` for the specific error |

---

## CI/CD

### GitHub Actions Workflows

Aegis uses three GitHub Actions workflows:

| Workflow | Trigger | Purpose |
|---|---|---|
| **CI** (`.github/workflows/ci.yml`) | Push to `main`, PRs | Lint + build UI, vet + build server, verify Docker build |
| **Release** (`.github/workflows/release-please.yml`) | Push to `main` | Auto-changelog via release-please, Docker push to GHCR on release |
| **Docs** (`.github/workflows/docs-pages.yml`) | Push to `main` (docs changes), manual | Build MkDocs site, deploy to GitHub Pages |

### Release Process

Releases are managed by [release-please](https://github.com/googleapis/release-please). The workflow:

1. Developers merge PRs into `main` using **Conventional Commits** (`feat:`, `fix:`, etc.)
2. release-please automatically creates/updates a "Release PR" with the changelog
3. When the Release PR is merged, it:
   - Bumps the version in `.release-please-manifest.json`
   - Updates `CHANGELOG.md`
   - Creates a GitHub Release with tag `v*`
   - Triggers the Docker image build and push to `ghcr.io/pixelvide/aegis`

### Docker Registry

Released images are pushed to GitHub Container Registry (GHCR):

```bash
# Pull latest release
docker pull ghcr.io/pixelvide/aegis:latest

# Pull specific version
docker pull ghcr.io/pixelvide/aegis:0.1.0
```

### Documentation Site

Docs are published to GitHub Pages at `https://pixelvide.github.io/aegis/` using MkDocs Material.

To preview docs locally:

```bash
pip install mkdocs-material
mkdocs serve
# Open http://127.0.0.1:8000
```

> **Setup required:** In the GitHub repo settings, go to **Settings → Pages → Source** and select **"GitHub Actions"**.
