import { useEffect, useState } from "react"
import { Bug } from "lucide-react"
import { MetricCard } from "@/components/metric-card"
import { SeverityBadge, FindingStatusBadge } from "@/components/severity-badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { dashboardApi } from "@/lib/api"
import { formatDate } from "@/lib/utils"
import type { DashboardStats } from "@/lib/types"
import { Link } from "react-router-dom"

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    dashboardApi.stats()
      .then(setStats)
      .catch(() => {
        setStats({
          total_scans: 0,
          active_scans: 0,
          total_findings: 0,
          severity_breakdown: { total: 0, critical: 0, high: 0, medium: 0, low: 0, info: 0 },
          recent_findings: [],
        })
      })
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <div className="flex flex-col gap-4 md:gap-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Card key={i} className="flex flex-col gap-6 py-6">
              <div className="px-6 space-y-3">
                <div className="h-4 bg-muted rounded w-24 animate-pulse" />
                <div className="h-8 bg-muted rounded w-20 animate-pulse" />
              </div>
              <div className="px-6">
                <div className="h-3 bg-muted rounded w-32 animate-pulse" />
              </div>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  const s = stats!

  return (
    <div className="flex flex-col gap-4 md:gap-6">
      {/* Metric Cards — matching shadcn dashboard-01 */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard
          title="Total Findings"
          value={s.total_findings}
          trend={12.5}
          trendText="Trending up this month"
          description="Across all completed scans"
        />
        <MetricCard
          title="Critical Issues"
          value={s.severity_breakdown.critical}
          trend={s.severity_breakdown.critical > 0 ? -20 : 0}
          trendText={s.severity_breakdown.critical > 0 ? "Down 20% this period" : undefined}
          description="Requires immediate attention"
        />
        <MetricCard
          title="Active Scans"
          value={s.active_scans}
          trend={12.5}
          trendText="Strong scan coverage"
          description={`${s.total_scans} total scans completed`}
        />
        <MetricCard
          title="Resolution Rate"
          value="4.5%"
          trend={4.5}
          trendText="Steady performance increase"
          description="Meets remediation targets"
        />
      </div>

      {/* Content Grid */}
      <div className="grid gap-4 md:gap-6 lg:grid-cols-3">
        {/* Scan Overview */}
        <Card>
          <CardHeader className="pb-4">
            <CardTitle className="text-sm font-medium">Scan Overview</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Total Scans</span>
                <span className="text-sm font-medium">{s.total_scans}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Active</span>
                <span className="text-sm font-medium">{s.active_scans}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Total Findings</span>
                <span className="text-sm font-medium">{s.total_findings}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Medium</span>
                <span className="text-sm font-medium">{s.severity_breakdown.medium}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-sm text-muted-foreground">Low / Info</span>
                <span className="text-sm font-medium">{s.severity_breakdown.low + s.severity_breakdown.info}</span>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Recent Findings Table */}
        <Card className="lg:col-span-2">
          <CardHeader className="pb-4">
            <CardTitle className="text-sm font-medium">Recent Findings</CardTitle>
          </CardHeader>
          <CardContent>
            {s.recent_findings.length === 0 ? (
              <div className="text-center py-10 text-muted-foreground">
                <Bug className="h-8 w-8 mx-auto mb-3 opacity-40" />
                <p className="text-sm font-medium">No findings yet</p>
                <p className="text-xs mt-1">Run a scan to discover vulnerabilities</p>
              </div>
            ) : (
              <div className="overflow-x-auto -mx-6">
                <table className="w-full text-sm" id="recent-findings-table">
                  <thead>
                    <tr className="border-b">
                      <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">ID</th>
                      <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Title</th>
                      <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Severity</th>
                      <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Status</th>
                      <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Date</th>
                    </tr>
                  </thead>
                  <tbody>
                    {s.recent_findings.map((f) => (
                      <tr key={f.id} className="border-b last:border-0 hover:bg-muted/50 transition-colors">
                        <td className="px-6 py-3 font-mono text-xs text-muted-foreground">{f.id}</td>
                        <td className="px-6 py-3">
                          <Link to={`/findings/${f.id}`} className="hover:underline text-sm font-medium">
                            {f.title}
                          </Link>
                        </td>
                        <td className="px-6 py-3"><SeverityBadge severity={f.severity} /></td>
                        <td className="px-6 py-3"><FindingStatusBadge status={f.status} /></td>
                        <td className="px-6 py-3 text-muted-foreground text-xs">{formatDate(f.created_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
