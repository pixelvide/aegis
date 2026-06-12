import { useEffect, useState, useCallback } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Key, Plus, Trash2 } from "lucide-react"
import { orgTokensApi, projectsApi } from "@/lib/api"
import type { APIToken, Project } from "@/lib/types"
import { formatDate } from "@/lib/utils"
import { CreateTokenDialog } from "@/components/create-token-dialog"

export default function ApiTokensPage() {
  const [tokens, setTokens] = useState<APIToken[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [revoking, setRevoking] = useState<string | null>(null)
  const [error, setError] = useState("")

  const loadTokens = useCallback(() => {
    setLoading(true)
    Promise.all([orgTokensApi.list(), projectsApi.list()])
      .then(([tokenData, projectData]) => {
        setTokens(tokenData || [])
        setProjects(projectData || [])
      })
      .catch(() => { setTokens([]); setProjects([]) })
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadTokens()
  }, [loadTokens])

  const projectName = (id?: string) => {
    if (!id) return "Org-wide"
    const p = projects.find((proj) => proj.id === id)
    return p ? p.name : id.slice(0, 8) + "..."
  }

  const handleRevoke = async (id: string) => {
    setRevoking(id)
    setError("")
    try {
      await orgTokensApi.revoke(id)
      loadTokens()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke token")
    } finally {
      setRevoking(null)
    }
  }

  return (
    <div className="flex flex-col gap-4 md:gap-6">
      <div>
        <h1 className="text-lg font-semibold">API Tokens</h1>
        <p className="text-sm text-muted-foreground">Manage API tokens for agent authentication</p>
      </div>

      <Card>
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Key className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-sm font-medium">All Tokens</CardTitle>
            </div>
            <Button size="sm" className="gap-2" onClick={() => setDialogOpen(true)} id="create-token-button">
              <Plus className="h-3.5 w-3.5" />
              Create Token
            </Button>
          </div>
          <CardDescription>
            Generate tokens for agent authentication. Tokens use <code className="text-xs">Authorization: Bearer aegis_xxx</code> and are scoped to this organization.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error && <p className="text-sm text-destructive mb-3">{error}</p>}

          {loading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <div key={i} className="flex items-center gap-3">
                  <div className="h-9 w-9 rounded bg-muted animate-pulse" />
                  <div className="flex-1 space-y-1.5">
                    <div className="h-4 bg-muted rounded w-32 animate-pulse" />
                    <div className="h-3 bg-muted rounded w-48 animate-pulse" />
                  </div>
                </div>
              ))}
            </div>
          ) : tokens.length === 0 ? (
            <div className="text-center py-10 text-muted-foreground">
              <Key className="h-8 w-8 mx-auto mb-3 opacity-40" />
              <p className="text-sm font-medium">No API tokens</p>
              <p className="text-xs mt-1">Create a token to authenticate your scanning agents.</p>
            </div>
          ) : (
            <div className="overflow-x-auto -mx-6">
              <table className="w-full text-sm" id="tokens-table">
                <thead>
                  <tr className="border-b">
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Name</th>
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Prefix</th>
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Scope</th>
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Expires</th>
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Status</th>
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Created</th>
                    <th className="px-6 pb-3 text-right text-xs font-medium text-muted-foreground"></th>
                  </tr>
                </thead>
                <tbody>
                  {tokens.map((t) => {
                    const isExpired = t.expires_at && new Date(t.expires_at) < new Date()
                    return (
                      <tr key={t.id} className="border-b last:border-0 hover:bg-muted/50 transition-colors">
                        <td className="px-6 py-3 font-medium">{t.name}</td>
                        <td className="px-6 py-3 font-mono text-xs text-muted-foreground">{t.prefix}...</td>
                        <td className="px-6 py-3">
                          <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
                            {projectName(t.project_id)}
                          </Badge>
                        </td>
                        <td className="px-6 py-3 text-xs text-muted-foreground">
                          {t.expires_at ? formatDate(t.expires_at) : "Never"}
                        </td>
                        <td className="px-6 py-3">
                          {t.revoked ? (
                            <Badge variant="outline" className="text-[10px] px-1.5 py-0 text-destructive border-destructive/30">Revoked</Badge>
                          ) : isExpired ? (
                            <Badge variant="outline" className="text-[10px] px-1.5 py-0 text-amber-600 border-amber-300">Expired</Badge>
                          ) : (
                            <Badge variant="outline" className="text-[10px] px-1.5 py-0 text-emerald-600 border-emerald-300">Active</Badge>
                          )}
                        </td>
                        <td className="px-6 py-3 text-xs text-muted-foreground">{formatDate(t.created_at)}</td>
                        <td className="px-6 py-3 text-right">
                          {!t.revoked && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-xs text-muted-foreground hover:text-destructive h-7"
                              onClick={() => handleRevoke(t.id)}
                              disabled={revoking === t.id}
                            >
                              <Trash2 className="h-3 w-3 mr-1" />
                              {revoking === t.id ? "Revoking..." : "Revoke"}
                            </Button>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <CreateTokenDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onCreated={loadTokens}
      />
    </div>
  )
}
