// API client for the Aegis Go server.
// All requests go through the Vite proxy (/api → 127.0.0.1:8080).

import type {
  Scan, Finding, Exploit, DashboardStats, CreateScanRequest,
  FindingStatus, Organization, Member, OrgRole, Project
} from "./types"

const BASE = "/api/v1"

// ─── Org Context ────────────────────────────────────────────────────────────
// Global org context — set by OrgProvider, read by all API calls.

let _currentOrgSlug: string | null = null
let _currentOrgId: string | null = null

export function setCurrentOrg(slug: string | null, id: string | null) {
  _currentOrgSlug = slug
  _currentOrgId = id
}

export function getCurrentOrg(): { slug: string | null; id: string | null } {
  return { slug: _currentOrgSlug, id: _currentOrgId }
}

// ─── Request Helper ─────────────────────────────────────────────────────────

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }

  // Inject org context header for tenant-scoped requests
  if (_currentOrgId) {
    headers["X-Org-ID"] = _currentOrgId
  } else if (_currentOrgSlug) {
    headers["X-Org-Slug"] = _currentOrgSlug
  }

  const res = await fetch(`${BASE}${path}`, {
    ...options,
    credentials: "include",
    headers: { ...headers, ...options?.headers as Record<string, string> },
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error || `Request failed: ${res.status}`)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

// ─── Organizations (no tenant context needed) ───────────────────────────────

export const orgsApi = {
  list: () => {
    // Org endpoints don't need X-Org-* headers, so use a clean fetch
    return fetch(`${BASE}/orgs`, {
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    }).then(r => {
      if (!r.ok) throw new Error("Failed to list orgs")
      return r.json()
    }) as Promise<Organization[]>
  },
  get: (slug: string) => {
    return fetch(`${BASE}/orgs/${encodeURIComponent(slug)}`, {
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    }).then(r => r.json()) as Promise<Organization>
  },
  create: (data: { name: string; slug: string; plan?: string }) => {
    return fetch(`${BASE}/orgs`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    }).then(r => r.json()) as Promise<Organization>
  },
}

// ─── Scans (tenant-scoped) ──────────────────────────────────────────────────

export const scansApi = {
  list: () => request<Scan[]>("/scans"),
  get: (id: string) => request<Scan>(`/scans/${encodeURIComponent(id)}`),
  create: (data: CreateScanRequest) =>
    request<Scan>("/scans", { method: "POST", body: JSON.stringify(data) }),
  cancel: (id: string) =>
    request<Scan>(`/scans/${encodeURIComponent(id)}/cancel`, { method: "POST" }),
  delete: (id: string) =>
    request<void>(`/scans/${encodeURIComponent(id)}`, { method: "DELETE" }),
}

// ─── Findings (tenant-scoped) ───────────────────────────────────────────────

export const findingsApi = {
  list: (params?: { scan_id?: string; severity?: string; status?: string; cwe?: string }) => {
    const qs = new URLSearchParams()
    if (params?.scan_id) qs.set("scan_id", params.scan_id)
    if (params?.severity) qs.set("severity", params.severity)
    if (params?.status) qs.set("status", params.status)
    if (params?.cwe) qs.set("cwe", params.cwe)
    const query = qs.toString()
    return request<Finding[]>(`/findings${query ? `?${query}` : ""}`)
  },
  get: (id: string) => request<Finding>(`/findings/${encodeURIComponent(id)}`),
  updateStatus: (id: string, status: FindingStatus) =>
    request<Finding>(`/findings/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    }),
  listExploits: (findingId: string) =>
    request<Exploit[]>(`/findings/${encodeURIComponent(findingId)}/exploits`),
  getExploit: (findingId: string, exploitId: string) =>
    request<Exploit>(
      `/findings/${encodeURIComponent(findingId)}/exploits/${encodeURIComponent(exploitId)}`
    ),
}

// ─── Dashboard (tenant-scoped) ──────────────────────────────────────────────

export const dashboardApi = {
  stats: () => request<DashboardStats>("/dashboard/stats"),
}

// ─── Members (tenant-scoped) ────────────────────────────────────────────────

export const membersApi = {
  list: () => request<Member[]>("/members"),
  invite: (email: string, role: OrgRole = "member") =>
    request<void>("/members/invite", {
      method: "POST",
      body: JSON.stringify({ email, role }),
    }),
  remove: (userId: string) =>
    request<void>(`/members/${encodeURIComponent(userId)}`, { method: "DELETE" }),
}

// ─── Projects (tenant-scoped) ───────────────────────────────────────────────

export const projectsApi = {
  list: () => request<Project[]>("/projects"),
  create: (data: { name: string; slug?: string }) =>
    request<Project>("/projects", { method: "POST", body: JSON.stringify(data) }),
  get: (slug: string) => request<Project>(`/projects/${encodeURIComponent(slug)}`),
}
