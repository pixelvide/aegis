// ─── Domain Types ───────────────────────────────────────────────────────────
// Mirrors the Go server models. Keep in sync.

export type ScanStatus = "pending" | "running" | "completed" | "failed" | "cancelled"
export type ExecMode = "direct" | "docker"
export type Severity = "critical" | "high" | "medium" | "low" | "info"
export type FindingStatus = "open" | "confirmed" | "fixed" | "false_positive" | "wontfix" | "verified"

export interface Target {
  type: "path" | "git" | "url"
  path?: string
  url?: string
  ref?: string
}

export interface Summary {
  total: number
  critical: number
  high: number
  medium: number
  low: number
  info: number
}

export interface Scan {
  id: string
  name: string
  target: Target
  persona: string
  mode: ExecMode
  status: ScanStatus
  prompt?: string
  agent_pid?: number
  conversation_id?: string
  workspace_path?: string
  finding_count: number
  summary?: Summary
  error_message?: string
  created_at: string
  started_at?: string
  completed_at?: string
}

export interface Finding {
  id: string
  scan_id: string
  project_id?: string
  fingerprint?: string
  title: string
  severity: Severity
  cwe?: string
  owasp?: string
  cve?: string
  cvss_score?: number
  cvss_vector?: string
  file?: string
  line?: number
  status: FindingStatus
  description: string
  source?: string
  seen_count: number
  last_seen_at: string
  exploits?: Exploit[]
  created_at: string
  updated_at: string
}

export interface Exploit {
  id: string
  finding_id: string
  filename: string
  language: string
  code: string
  validated: boolean
}

export interface DashboardStats {
  total_scans: number
  active_scans: number
  total_findings: number
  severity_breakdown: Summary
  recent_findings: Finding[]
}

export interface CreateScanRequest {
  name?: string
  target: Target
  persona: string
  mode: ExecMode
  prompt?: string
}

// ─── Multi-Tenant Types ─────────────────────────────────────────────────────

export interface Organization {
  id: string
  name: string
  slug: string
  plan: string
  created_at: string
}

export interface Project {
  id: string
  name: string
  slug: string
  created_at: string
}

export type OrgRole = "owner" | "admin" | "member" | "viewer"

export interface Member {
  user_id: string
  email: string
  name: string
  avatar_url: string
  role: OrgRole
}

// ─── API Token Types ────────────────────────────────────────────────────────

export interface APIToken {
  id: string
  project_id?: string
  name: string
  prefix: string
  created_by: string
  last_used?: string
  expires_at?: string
  revoked: boolean
  created_at: string
}

export interface CreateTokenResponse {
  token: string   // plaintext, shown once
  info: APIToken
}
