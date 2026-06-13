import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { ScanStatusBadge } from "@/components/severity-badge"
import { Card, CardContent } from "@/components/ui/card"
import { scansApi } from "@/lib/api"
import { useProject } from "@/lib/project-context"
import { timeAgo } from "@/lib/utils"
import type { Scan } from "@/lib/types"
import { Search, Eye, Zap, FlaskConical } from "lucide-react"

const personaIcons: Record<string, { icon: typeof Eye; label: string; color: string }> = {
  sharingan: { icon: Eye, label: "Sharingan", color: "text-red-500" },
  killua: { icon: Zap, label: "Killua", color: "text-blue-500" },
  senku: { icon: FlaskConical, label: "Senku", color: "text-green-500" },
}

export default function ScansPage() {
  const [scans, setScans] = useState<Scan[]>([])
  const [loading, setLoading] = useState(true)
  const { currentProject } = useProject()

  useEffect(() => {
    scansApi.list()
      .then(setScans)
      .catch(() => setScans([]))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Scans</h1>
        <span className="text-sm text-muted-foreground">{scans.length} total</span>
      </div>

      {loading ? (
        <div className="space-y-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-20 bg-muted/50 rounded-lg animate-pulse" />
          ))}
        </div>
      ) : scans.length === 0 ? (
        <div className="text-center py-20">
          <Search className="h-12 w-12 mx-auto mb-4 text-muted-foreground/40" />
          <p className="font-medium text-lg">No scans yet</p>
          <p className="text-sm text-muted-foreground mt-1">Create a scan to start analyzing your application</p>
        </div>
      ) : (
        <Card>
          <CardContent className="p-0">
            <table className="w-full text-sm" id="scans-table">
              <thead>
                <tr className="border-b">
                  <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Name</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Target</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Persona</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Status</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Findings</th>
                  <th className="px-4 py-3 text-left text-xs font-medium text-muted-foreground">Created</th>
                </tr>
              </thead>
              <tbody>
                {scans.map((scan) => {
                  const persona = personaIcons[scan.persona] || personaIcons.sharingan
                  const PersonaIcon = persona.icon
                  return (
                    <tr key={scan.id} className="border-b last:border-0 hover:bg-muted/50 transition-colors">
                      <td className="px-4 py-3">
                        <Link
                          to={currentProject ? `/project/${currentProject.id}/findings?scan_id=${scan.id}` : '#'}
                          className="font-medium hover:underline hover:text-blue-600"
                        >
                          {scan.name}
                        </Link>
                      </td>
                      <td className="px-4 py-3 font-mono text-xs text-muted-foreground max-w-[200px] truncate">
                        {scan.target.path || scan.target.url || "—"}
                      </td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex items-center gap-1.5 text-xs font-medium ${persona.color}`}>
                          <PersonaIcon className="h-3.5 w-3.5" />
                          {persona.label}
                        </span>
                      </td>
                      <td className="px-4 py-3"><ScanStatusBadge status={scan.status} /></td>
                      <td className="px-4 py-3 font-medium">{scan.finding_count}</td>
                      <td className="px-4 py-3 text-xs text-muted-foreground">{timeAgo(scan.created_at)}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
