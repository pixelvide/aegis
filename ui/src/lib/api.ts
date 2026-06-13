// API client for the Aegis Go server.
// All requests go through the Vite proxy (/api → 127.0.0.1:8080).

import type {
  Scan, Finding, Exploit, DashboardStats, CreateScanRequest,
  FindingStatus, Organization, Member, OrgRole, Project,
  APIToken, CreateTokenResponse,
} from "./types"
import type { ErrorType, ErrorCode } from "./error-codes.gen"

const BASE = "/api/v1"

// ─── Org Context ────────────────────────────────────────────────────────────
// Global org context — set by OrgProvider, read by all API calls.

let _currentOrgSlug: string | null = null
let _currentOrgId: string | null = null
let _currentProjectId: string | null = null

export function setCurrentOrg(slug: string | null, id: string | null) {
  _currentOrgSlug = slug
  _currentOrgId = id
}

export function getCurrentOrg(): { slug: string | null; id: string | null } {
  return { slug: _currentOrgSlug, id: _currentOrgId }
}

export function setCurrentProject(id: string | null) {
  _currentProjectId = id
}

export function getCurrentProject(): string | null {
  return _currentProjectId
}

// ─── Error Types ────────────────────────────────────────────────────────────

/** A single structured error from the API. */
export interface ApiErrorDetail {
  type: ErrorType
  code: ErrorCode
  ref: string
  message: string
  details?: { field: string; message: string }[]
}

/** Custom error class for API errors with structured error codes. */
export class ApiError extends Error {
  /** The first (primary) error's type category */
  readonly type: ErrorType
  /** The first (primary) error's machine-readable code */
  readonly code: ErrorCode
  /** The first (primary) error's reference code (e.g. E10001) */
  readonly ref: string
  /** Request ID for support / debugging */
  readonly requestId: string
  /** All errors returned by the API */
  readonly errors: ApiErrorDetail[]

  constructor(errors: ApiErrorDetail[], requestId: string) {
    const primary = errors[0]
    super(primary?.message || "Unknown error")
    this.name = "ApiError"
    this.type = primary?.type || ("server_error" as ErrorType)
    this.code = primary?.code || ("internal" as ErrorCode)
    this.ref = primary?.ref || "E90001"
    this.requestId = requestId
    this.errors = errors
  }

  /** Check if this error matches a specific error code. */
  is(code: ErrorCode): boolean {
    return this.code === code
  }

  /** Check if this error matches a specific error type category. */
  isType(type: ErrorType): boolean {
    return this.type === type
  }
}

// ─── Response Types ─────────────────────────────────────────────────────────

/** Pagination metadata from list endpoints (Cloudflare-style result_info). */
export interface ResultInfo {
  page: number
  per_page: number
  total: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
}

/** Response from paginated list endpoints. */
export interface ListResponse<T> {
  result: T[]
  result_info: ResultInfo
}

// ─── Request Helper ─────────────────────────────────────────────────────────

/**
 * Core request helper. Handles both the new Cloudflare-style envelope
 * ({success, result, errors}) and legacy format ({error: "..."}) for
 * backward compatibility during the phased migration.
 */
export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }

  // Inject org context header for tenant-scoped requests
  if (_currentOrgId) {
    headers["X-Org-ID"] = _currentOrgId
  }

  const res = await fetch(`${BASE}${path}`, {
    ...options,
    credentials: "include",
    headers: { ...headers, ...options?.headers as Record<string, string> },
  })

  if (!res.ok) {
    // Auto-logout on 401 (expired/invalid JWT) — skip auth endpoints to avoid redirect loops
    const isAuthEndpoint = path.startsWith("/auth/")
    if (res.status === 401 && !isAuthEndpoint) {
      // Clear cookie and redirect to login
      await fetch(`${BASE}/auth/logout`, { method: "POST", credentials: "include" }).catch(() => {})
      window.location.href = "/login"
      throw new Error("Session expired")
    }

    const body = await res.json().catch(() => ({ error: res.statusText }))

    // New envelope format: { success: false, errors: [...], request_id: "..." }
    if (body.success === false && Array.isArray(body.errors)) {
      throw new ApiError(body.errors, body.request_id || "")
    }

    // Legacy format: { error: "message" }
    throw new Error(body.error || body.message || `Request failed: ${res.status}`)
  }

  if (res.status === 204) return undefined as T

  const body = await res.json()

  // New envelope format: unwrap { success: true, result: ... }
  if (body.success === true && "result" in body) {
    return body.result as T
  }

  // Legacy format: return raw body
  return body as T
}

/**
 * Request helper for paginated list endpoints.
 * Returns the result array and pagination metadata (result_info).
 */
export async function requestList<T>(
  path: string,
  params?: { page?: number; per_page?: number },
  options?: RequestInit,
): Promise<ListResponse<T>> {
  const qs = new URLSearchParams()
  if (params?.page) qs.set("page", String(params.page))
  if (params?.per_page) qs.set("per_page", String(params.per_page))
  const query = qs.toString()
  const fullPath = query ? `${path}${path.includes("?") ? "&" : "?"}${query}` : path

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }
  if (_currentOrgId) {
    headers["X-Org-ID"] = _currentOrgId
  }

  const res = await fetch(`${BASE}${fullPath}`, {
    ...options,
    credentials: "include",
    headers: { ...headers, ...options?.headers as Record<string, string> },
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    if (body.success === false && Array.isArray(body.errors)) {
      throw new ApiError(body.errors, body.request_id || "")
    }
    throw new Error(body.error || `Request failed: ${res.status}`)
  }

  const body = await res.json()

  // New envelope: { success: true, result: [...], result_info: {...} }
  if (body.success === true && "result" in body && "result_info" in body) {
    return { result: body.result as T[], result_info: body.result_info }
  }

  // Legacy: raw array — wrap in a synthetic ListResponse
  const items = Array.isArray(body) ? body : (body.result || [])
  return {
    result: items as T[],
    result_info: {
      page: 1,
      per_page: items.length,
      total: items.length,
      total_pages: 1,
      has_next: false,
      has_prev: false,
    },
  }
}

// ─── Organizations (no tenant context needed) ───────────────────────────────

export interface OrgsListResponse {
  orgs: Organization[]
  base_domain: string
}

export const orgsApi = {
  list: () => {
    // Org endpoints don't need X-Org-* headers, so use a clean fetch
    return fetch(`${BASE}/orgs`, {
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    }).then(r => {
      if (!r.ok) throw new Error("Failed to list orgs")
      return r.json()
    }).then(body => (body.result ?? body) as OrgsListResponse)
  },
  get: (slug: string) => {
    return fetch(`${BASE}/orgs/${encodeURIComponent(slug)}`, {
      credentials: "include",
      headers: { "Content-Type": "application/json" },
    }).then(r => r.json()).then(body => (body.result ?? body) as Organization)
  },
  create: async (data: { name: string; slug?: string; plan?: string }): Promise<Organization> => {
    const res = await fetch(`${BASE}/orgs`, {
      method: "POST",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({ errors: [{ message: "Failed to create organization" }] }))
      throw new Error(body.errors?.[0]?.message || body.error || `Request failed: ${res.status}`)
    }
    const body = await res.json()
    return (body.result ?? body) as Organization
  },

}

// ─── Scans (tenant-scoped) ──────────────────────────────────────────────────

export const scansApi = {
  list: () => {
    if (!_currentProjectId) throw new Error("No project selected")
    return request<Scan[]>(`/projects/${_currentProjectId}/scans`)
  },
  get: (id: string) => {
    if (!_currentProjectId) throw new Error("No project selected")
    return request<Scan>(`/projects/${_currentProjectId}/scans/${encodeURIComponent(id)}`)
  },
  create: (data: CreateScanRequest) => {
    const projectId = data.project_id || _currentProjectId
    if (!projectId) throw new Error("No project selected")
    const payload = { ...data, project_id: projectId }
    return request<Scan>(`/projects/${projectId}/scans`, { method: "POST", body: JSON.stringify(payload) })
  },
  cancel: (id: string) => {
    if (!_currentProjectId) throw new Error("No project selected")
    return request<Scan>(`/projects/${_currentProjectId}/scans/${encodeURIComponent(id)}/cancel`, { method: "POST" })
  },
  delete: (id: string) => {
    if (!_currentProjectId) throw new Error("No project selected")
    return request<void>(`/projects/${_currentProjectId}/scans/${encodeURIComponent(id)}`, { method: "DELETE" })
  },
}

// ─── Findings (tenant-scoped) ───────────────────────────────────────────────

export const findingsApi = {
  list: (params?: { scan_id?: string; project_id?: string; severity?: string; status?: string; cwe?: string }) => {
    const projectId = params?.project_id || _currentProjectId
    if (!projectId) throw new Error("No project selected")
    
    const qs = new URLSearchParams()
    if (params?.scan_id) qs.set("scan_id", params.scan_id)
    if (params?.severity) qs.set("severity", params.severity)
    if (params?.status) qs.set("status", params.status)
    if (params?.cwe) qs.set("cwe", params.cwe)
    const query = qs.toString()
    return request<Finding[]>(`/projects/${projectId}/findings${query ? `?${query}` : ""}`)
  },
  get: (id: string) => {
    if (!_currentProjectId) throw new Error("No project selected")
    return request<Finding>(`/projects/${_currentProjectId}/findings/${encodeURIComponent(id)}`)
  },
  updateStatus: (id: string, status: FindingStatus) => {
    if (!_currentProjectId) throw new Error("No project selected")
    return request<Finding>(`/projects/${_currentProjectId}/findings/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    })
  },
  listExploits: (findingId: string) => {
    if (!_currentProjectId) throw new Error("No project selected")
    return request<Exploit[]>(`/projects/${_currentProjectId}/findings/${encodeURIComponent(findingId)}/exploits`)
  },
  getExploit: (findingId: string, exploitId: string) => {
    if (!_currentProjectId) throw new Error("No project selected")
    return request<Exploit>(
      `/projects/${_currentProjectId}/findings/${encodeURIComponent(findingId)}/exploits/${encodeURIComponent(exploitId)}`
    )
  },
}

// ─── Dashboard (tenant-scoped) ──────────────────────────────────────────────

export const dashboardApi = {
  stats: () => {
    if (_currentProjectId) {
      return request<DashboardStats>(`/projects/${_currentProjectId}/dashboard/stats`)
    }
    return request<DashboardStats>("/dashboard/stats")
  },
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

// ─── Project Tokens (tenant-scoped, per-project) ────────────────────────────

export const projectTokensApi = {
  list: (projectId?: string) => {
    const pid = projectId || _currentProjectId
    if (!pid) throw new Error("No project selected")
    return request<APIToken[]>(`/projects/${pid}/tokens`)
  },
  create: (data: { name: string; expires_in?: number }, projectId?: string) => {
    const pid = projectId || _currentProjectId
    if (!pid) throw new Error("No project selected")
    return request<CreateTokenResponse>(`/projects/${pid}/tokens`, {
      method: "POST",
      body: JSON.stringify(data),
    })
  },
  revoke: (tokenId: string, projectId?: string) => {
    const pid = projectId || _currentProjectId
    if (!pid) throw new Error("No project selected")
    return request<void>(`/projects/${pid}/tokens/${encodeURIComponent(tokenId)}`, {
      method: "DELETE",
    })
  },
}

// ─── Org Tokens (tenant-scoped, org-wide, admin/owner) ──────────────────────

export const orgTokensApi = {
  list: () => request<APIToken[]>("/tokens"),
  create: (data: { name: string; expires_in?: number }) =>
    request<CreateTokenResponse>("/tokens", {
      method: "POST",
      body: JSON.stringify(data),
    }),
  revoke: (id: string) =>
    request<void>(`/tokens/${encodeURIComponent(id)}`, { method: "DELETE" }),
}

// ─── Org Feature Flags (tenant-scoped) ──────────────────────────────────────

export interface OrgFeatureFlag {
  name: string
  provisioned: boolean
  enabled: boolean
  description: string
}

export const orgFeaturesApi = {
  list: () => request<OrgFeatureFlag[]>("/org-features"),
  update: (flag: string, enabled: boolean) =>
    request<void>(`/org-features/${encodeURIComponent(flag)}`, {
      method: "PATCH",
      body: JSON.stringify({ enabled }),
    }),
}

// Legacy alias for backward compatibility
export const tokensApi = projectTokensApi
