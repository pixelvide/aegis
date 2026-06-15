import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { ScanStatusBadge } from "@/components/severity-badge"
import { scansApi } from "@/lib/api"
import { useProject } from "@/lib/project-context"
import { timeAgo } from "@/lib/utils"
import type { Scan, Summary } from "@/lib/types"
import { Search, Eye, Zap, FlaskConical, Clock, AlertTriangle, Bug } from "lucide-react"

// ── Persona mapping ─────────────────────────────────────────────────────────

const personaConfig: Record<string, { icon: typeof Eye; label: string; color: string; bgColor: string }> = {
  sharingan: { icon: Eye, label: "Sharingan", color: "text-red-500", bgColor: "bg-red-500/10" },
  killua: { icon: Zap, label: "Killua", color: "text-blue-500", bgColor: "bg-blue-500/10" },
  senku: { icon: FlaskConical, label: "Senku", color: "text-green-500", bgColor: "bg-green-500/10" },
}

// ── Severity bar colors ─────────────────────────────────────────────────────

const severityColors: Record<string, string> = {
  critical: "bg-red-600",
  high: "bg-orange-500",
  medium: "bg-amber-400",
  low: "bg-blue-400",
  info: "bg-gray-400",
}

const severityLabels: Record<string, string> = {
  critical: "Critical",
  high: "High",
  medium: "Medium",
  low: "Low",
  info: "Info",
}

// ── Helpers ─────────────────────────────────────────────────────────────────

function formatDuration(startedAt?: string, completedAt?: string): string {
  if (!startedAt || !completedAt) return ""
  const start = new Date(startedAt).getTime()
  const end = new Date(completedAt).getTime()
  const diffMs = end - start
  if (diffMs < 0) return ""

  const seconds = Math.floor(diffMs / 1000)
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  const remainingSeconds = seconds % 60
  if (minutes < 60) return `${minutes}m ${remainingSeconds}s`
  const hours = Math.floor(minutes / 60)
  const remainingMinutes = minutes % 60
  return `${hours}h ${remainingMinutes}m`
}

function isStale(scan: Scan): boolean {
  if (scan.status !== "running") return false
  if (!scan.started_at) return false
  const twoHoursMs = 2 * 60 * 60 * 1000
  return Date.now() - new Date(scan.started_at).getTime() > twoHoursMs
}

// ── Severity Bar ────────────────────────────────────────────────────────────

function SeverityBar({ summary }: { summary?: Summary }) {
  if (!summary || summary.total === 0) {
    return (
      <div className="flex items-center gap-2">
        <div className="flex-1 h-2 rounded-full bg-muted/50" />
        <span className="text-xs text-muted-foreground">No findings</span>
      </div>
    )
  }

  const segments = [
    { key: "critical", count: summary.critical },
    { key: "high", count: summary.high },
    { key: "medium", count: summary.medium },
    { key: "low", count: summary.low },
    { key: "info", count: summary.info },
  ].filter(s => s.count > 0)

  return (
    <div className="space-y-1.5">
      {/* Stacked bar */}
      <div className="flex h-2 rounded-full overflow-hidden bg-muted/30">
        {segments.map(seg => (
          <div
            key={seg.key}
            className={`${severityColors[seg.key]} transition-all duration-500`}
            style={{ width: `${(seg.count / summary.total) * 100}%` }}
            title={`${severityLabels[seg.key]}: ${seg.count}`}
          />
        ))}
      </div>
      {/* Legend */}
      <div className="flex items-center gap-3 text-[11px] text-muted-foreground">
        {segments.map(seg => (
          <span key={seg.key} className="flex items-center gap-1">
            <span className={`h-2 w-2 rounded-full ${severityColors[seg.key]}`} />
            {seg.count} {severityLabels[seg.key]}
          </span>
        ))}
      </div>
    </div>
  )
}

// ── Scan Card ───────────────────────────────────────────────────────────────

function ScanCard({ scan }: { scan: Scan }) {
  const { currentProject } = useProject()
  const persona = personaConfig[scan.persona] || personaConfig.sharingan
  const PersonaIcon = persona.icon
  const stale = isStale(scan)
  const duration = formatDuration(scan.started_at, scan.completed_at)
  const linkTo = currentProject
    ? `/project/${currentProject.id}/findings?scan_id=${encodeURIComponent(scan.id)}`
    : "#"

  return (
    <Link
      to={linkTo}
      id={`scan-card-${scan.id}`}
      className="block group"
    >
      <div className="rounded-lg border bg-card hover:bg-accent/50 transition-all duration-200 hover:shadow-sm p-4 space-y-3">
        {/* Header row: persona icon + name + status + time */}
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-3 min-w-0">
            <div className={`flex-shrink-0 h-9 w-9 rounded-lg ${persona.bgColor} flex items-center justify-center`}>
              <PersonaIcon className={`h-4.5 w-4.5 ${persona.color}`} />
            </div>
            <div className="min-w-0">
              <h3 className="text-sm font-medium truncate group-hover:text-foreground">
                {scan.name}
              </h3>
              <div className="flex items-center gap-2 mt-0.5">
                <span className={`text-xs font-medium ${persona.color}`}>
                  {persona.label}
                </span>
                {scan.target.path && (
                  <span className="text-[11px] text-muted-foreground font-mono truncate max-w-[200px]">
                    {scan.target.path}
                  </span>
                )}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 flex-shrink-0">
            {stale && (
              <span className="inline-flex items-center gap-1 text-[11px] text-amber-600 dark:text-amber-400 bg-amber-500/10 px-2 py-0.5 rounded-full">
                <AlertTriangle className="h-3 w-3" />
                Stale
              </span>
            )}
            <ScanStatusBadge status={scan.status} />
          </div>
        </div>

        {/* Severity breakdown bar */}
        <SeverityBar summary={scan.summary} />

        {/* Footer: finding count + duration + time ago */}
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-1">
              <Bug className="h-3.5 w-3.5" />
              {scan.finding_count} {scan.finding_count === 1 ? "finding" : "findings"}
            </span>
            {duration && (
              <span className="flex items-center gap-1">
                <Clock className="h-3.5 w-3.5" />
                {duration}
              </span>
            )}
          </div>
          <span>{timeAgo(scan.created_at)}</span>
        </div>

        {/* Error message for failed scans */}
        {scan.status === "failed" && scan.error_message && (
          <div className="text-xs text-red-600 dark:text-red-400 bg-red-500/5 border border-red-500/10 rounded px-3 py-1.5 truncate">
            {scan.error_message}
          </div>
        )}
      </div>
    </Link>
  )
}

// ── Loading Skeleton ────────────────────────────────────────────────────────

function ScanCardSkeleton() {
  return (
    <div className="rounded-lg border bg-card p-4 space-y-3 animate-pulse">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className="h-9 w-9 rounded-lg bg-muted" />
          <div className="space-y-1.5">
            <div className="h-4 w-40 bg-muted rounded" />
            <div className="h-3 w-24 bg-muted rounded" />
          </div>
        </div>
        <div className="h-5 w-20 bg-muted rounded-full" />
      </div>
      <div className="h-2 rounded-full bg-muted" />
      <div className="flex justify-between">
        <div className="h-3 w-24 bg-muted rounded" />
        <div className="h-3 w-16 bg-muted rounded" />
      </div>
    </div>
  )
}

// ── Main Page ───────────────────────────────────────────────────────────────

export default function ScansPage() {
  const [scans, setScans] = useState<Scan[]>([])
  const [loading, setLoading] = useState(true)
  const { currentProject } = useProject()

  useEffect(() => {
    if (!currentProject) return
    scansApi.list()
      .then(data => {
        // Sort newest first (by created_at descending)
        const sorted = [...data].sort(
          (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
        )
        setScans(sorted)
      })
      .catch(() => setScans([]))
      .finally(() => setLoading(false))
  }, [currentProject])

  if (!currentProject) {
    return (
      <div className="text-center py-20">
        <Search className="h-12 w-12 mx-auto mb-4 text-muted-foreground/40" />
        <p className="font-medium text-lg">Select a project</p>
        <p className="text-sm text-muted-foreground mt-1">
          Choose a project from the sidebar to view its scans
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-6" id="scans-page">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Scans</h1>
          <p className="text-sm text-muted-foreground">
            Agent-submitted security scans for this project
          </p>
        </div>
        {!loading && scans.length > 0 && (
          <span className="text-sm text-muted-foreground">{scans.length} total</span>
        )}
      </div>

      {loading ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <ScanCardSkeleton key={i} />
          ))}
        </div>
      ) : scans.length === 0 ? (
        <div className="text-center py-20">
          <Search className="h-12 w-12 mx-auto mb-4 text-muted-foreground/40" />
          <p className="font-medium text-lg">No scans yet</p>
          <p className="text-sm text-muted-foreground mt-1 max-w-sm mx-auto">
            Run the Aegis agent with this project's API token to start scanning.
            Scans will appear here automatically.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {scans.map(scan => (
            <ScanCard key={scan.id} scan={scan} />
          ))}
        </div>
      )}
    </div>
  )
}
