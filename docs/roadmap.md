# Aegis Roadmap

Last updated: 2026-06-11

This document tracks planned features, improvements, and technical debt for the Aegis platform. Items are organized by priority and grouped into phases.

---

## Phase 1 — Core UI Completeness

High-priority items to make the existing platform fully functional end-to-end.

### Token Management UI
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** Create a settings tab (or dedicated page) for managing API tokens. Users need to create, view, and revoke tokens from the UI.
- **Requirements:**
  - Token creation form (name, project scope, expiry)
  - Show plaintext token once after creation (with copy button)
  - List tokens with prefix, name, created date, last used, expiry
  - Revoke button with confirmation

### Finding Detail Enhancements
- **Status:** Not started
- **Priority:** 🔴 Critical
- **Description:** Update finding detail page to display new fields: fingerprint, CVE/CVSS, source, seen count, verification status.
- **Requirements:**
  - Show CVE link (to NVD) and CVSS score badge
  - Display `seen_count` and `last_seen_at` for dedup visibility
  - Show `verified` status with visual indicator
  - Show `source` tag (e.g., CI run ID)

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

## Phase 4 — Access Control & Security

### Role-Based Access Control (RBAC)
- **Status:** Not started
- **Priority:** 🟡 Medium
- **Description:** Roles (`owner`, `admin`, `member`, `viewer`) exist in the data model but aren't enforced in any handler.
- **Requirements:**
  - `viewer` — read-only access to scans/findings
  - `member` — create scans, triage findings
  - `admin` — manage members, delete scans, manage projects
  - `owner` — full access including org deletion and role management
  - Middleware or per-handler checks
  - UI should hide/disable actions based on role

### Audit Log
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** Log security-relevant actions (member invite/remove, scan create/delete, finding triage changes) for compliance.
- **Changes needed:**
  - `audit_log` table in per-org schema
  - Store interface + postgres implementation
  - Write audit entries from handlers
  - UI page to browse audit log

---

## Phase 5 — Integrations

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
- **Description:** Allow orgs to configure webhooks that fire when findings are created, status changes, or agents complete a scan pass.
- **Requirements:**
  - `org_xxx.webhooks` table (URL, events, secret)
  - HMAC-signed POST to webhook URL
  - Retry with exponential backoff

---

## Phase 6 — User Experience

### Notifications System
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** The sidebar user menu has a "Notifications" item but nothing exists. Notify users when scans complete or critical findings are discovered.

### User Account Page
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** The sidebar user menu has "Account" and "Billing" items but they don't navigate anywhere. Need profile editing (name, avatar, password change).

### Search / Command Palette
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** The sidebar has a "Search" button at the bottom. Implement a global search / command palette (Cmd+K) to quickly navigate to scans, findings, or settings.

### Dark Mode Toggle
- **Status:** Not started
- **Priority:** 🟢 Low
- **Description:** The UI uses shadcn/ui which supports dark mode theming. Add a toggle in settings or the user menu.

---

## Technical Debt

| Item | Description | Priority |
|---|---|---|
| Error boundaries | No React error boundaries — unhandled errors crash the whole app | 🟡 Medium |
| Loading states | Some pages show raw empty states before data loads | 🟢 Low |
| API pagination | `ListFindings` returns all records — needs pagination for scale | 🟡 Medium |
| Test suite | No unit tests or integration tests exist | 🟡 Medium |
| Rate limiting | No rate limiting on auth or agent ingest endpoints | 🟡 Medium |
| OpenAPI spec update | Swagger spec needs agent ingest + token endpoints added | 🟡 Medium |

---

## Completed ✅

| Feature | Date |
|---|---|
| Auth system (register, login, logout, JWT sessions) | Done |
| Multi-tenant architecture (per-org PostgreSQL schemas) | Done |
| Organization CRUD + auto-provisioning | Done |
| Schema migration system | Done |
| Feature flags | Done |
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

