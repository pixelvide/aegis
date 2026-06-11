import { useEffect, useState, useCallback } from "react"
import { Link } from "react-router-dom"
import { SeverityBadge, FindingStatusBadge } from "@/components/severity-badge"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Bug, Filter, ArrowUpDown } from "lucide-react"
import { findingsApi, projectsApi } from "@/lib/api"
import { formatDate } from "@/lib/utils"
import type { Finding, Severity, FindingStatus, Project } from "@/lib/types"

export default function FindingsPage() {
  const [findings, setFindings] = useState<Finding[]>([])
  const [loading, setLoading] = useState(true)
  const [severityFilter, setSeverityFilter] = useState<string>("")
  const [statusFilter, setStatusFilter] = useState<string>("")
  const [projectFilter, setProjectFilter] = useState<string>("")
  const [projects, setProjects] = useState<Project[]>([])

  const fetchFindings = useCallback((severity: string, status: string, projectId: string) => {
    findingsApi.list({
      severity: severity || undefined,
      status: status || undefined,
      project_id: projectId || undefined,
    })
      .then(setFindings)
      .catch(() => setFindings([]))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    projectsApi.list().then(setProjects).catch(() => setProjects([]))
  }, [])

  useEffect(() => {
    fetchFindings(severityFilter, statusFilter, projectFilter)
  }, [fetchFindings, severityFilter, statusFilter, projectFilter])

  const severities: Severity[] = ["critical", "high", "medium", "low", "info"]
  const statuses: FindingStatus[] = ["open", "confirmed", "fixed", "false_positive", "wontfix", "verified"]

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Findings</h1>
        <span className="text-sm text-muted-foreground">{findings.length} total</span>
      </div>

      {/* Filters */}
      <div className="flex gap-2 flex-wrap" id="findings-filters">
        <div className="flex items-center gap-2">
          <Filter className="h-4 w-4 text-muted-foreground" />
          <select
            value={severityFilter}
            onChange={(e) => { setLoading(true); setSeverityFilter(e.target.value) }}
            className="rounded-md border bg-background px-3 py-1.5 text-sm"
            id="severity-filter"
          >
            <option value="">All Severities</option>
            {severities.map((s) => (
              <option key={s} value={s}>{s.charAt(0).toUpperCase() + s.slice(1)}</option>
            ))}
          </select>
          <select
            value={statusFilter}
            onChange={(e) => { setLoading(true); setStatusFilter(e.target.value) }}
            className="rounded-md border bg-background px-3 py-1.5 text-sm"
            id="status-filter"
          >
            <option value="">All Statuses</option>
            {statuses.map((s) => (
              <option key={s} value={s}>{s.replace("_", " ")}</option>
            ))}
          </select>
          <select
            value={projectFilter}
            onChange={(e) => { setLoading(true); setProjectFilter(e.target.value) }}
            className="rounded-md border bg-background px-3 py-1.5 text-sm"
            id="project-filter"
          >
            <option value="">All Projects</option>
            {projects.map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
          {(severityFilter || statusFilter || projectFilter) && (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => { setLoading(true); setSeverityFilter(""); setStatusFilter(""); setProjectFilter("") }}
            >
              Clear
            </Button>
          )}
        </div>
      </div>

      {/* Findings Table */}
      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="p-8 space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-12 bg-muted/50 rounded animate-pulse" />
              ))}
            </div>
          ) : findings.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <Bug className="h-12 w-12 mx-auto mb-4 opacity-40" />
              <p className="font-medium text-lg">No findings found</p>
              <p className="text-sm mt-1">
                {severityFilter || statusFilter
                  ? "Try adjusting your filters"
                  : "Run a scan to discover vulnerabilities"}
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm" id="findings-table">
                <thead>
                  <tr className="border-b">
                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">
                      <span className="flex items-center gap-1">ID <ArrowUpDown className="h-3 w-3" /></span>
                    </th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Title</th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Severity</th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">CWE</th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">File</th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Status</th>
                    <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Date</th>
                  </tr>
                </thead>
                <tbody>
                  {findings.map((f) => (
                    <tr
                      key={f.id}
                      className="border-b last:border-0 hover:bg-muted/50 transition-colors cursor-pointer"
                    >
                      <td className="px-4 py-3 font-mono text-xs">{f.id}</td>
                      <td className="px-4 py-3">
                        <Link
                          to={`/findings/${f.id}`}
                          className="font-medium hover:underline hover:text-blue-600 transition-colors"
                        >
                          {f.title}
                        </Link>
                      </td>
                      <td className="px-4 py-3"><SeverityBadge severity={f.severity} /></td>
                      <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{f.cwe || "—"}</td>
                      <td className="px-4 py-3 font-mono text-xs text-muted-foreground max-w-[200px] truncate">
                        {f.file || "—"}
                      </td>
                      <td className="px-4 py-3"><FindingStatusBadge status={f.status} /></td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{formatDate(f.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
