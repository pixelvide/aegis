import { useEffect, useState, useCallback, useMemo } from "react"
import { useParams, NavLink } from "react-router-dom"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Key, Plus, Trash2, Filter, Globe, FolderKey, ArrowLeft, Copy, Check } from "lucide-react"
import { orgTokensApi, projectTokensApi, projectsApi } from "@/lib/api"
import type { APIToken, Project } from "@/lib/types"
import { formatDate, copyToClipboard } from "@/lib/utils"
import { CreateTokenDialog } from "@/components/create-token-dialog"
import { useOrg } from "@/lib/org-context"

type ScopeFilter = "all" | "org-wide" | string // string = project ID

interface ApiTokensPageProps {
  /** When set, page is project-scoped. When absent, reads from route params or defaults to org-wide. */
  projectId?: string
}

export default function ApiTokensPage({ projectId: propProjectId }: ApiTokensPageProps = {}) {
  const params = useParams<{ projectId: string }>()
  const projectId = propProjectId || params.projectId
  const isProjectScoped = !!projectId
  const { currentOrg, baseDomain } = useOrg()

  const [tokens, setTokens] = useState<APIToken[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [project, setProject] = useState<Project | null>(null)
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [revoking, setRevoking] = useState<string | null>(null)
  const [error, setError] = useState("")
  const [scopeFilter, setScopeFilter] = useState<ScopeFilter>("all")
  const [copiedEnv, setCopiedEnv] = useState(false)

  // Compute the base URL for the org
  const getBaseUrl = () => {
    const protocol = window.location.protocol
    if (baseDomain && currentOrg) {
      const port = window.location.port ? `:${window.location.port}` : ""
      return `${protocol}//${currentOrg.slug}.${baseDomain}${port}`
    }
    return window.location.origin
  }

  // Build the .env content string (for the persistent snippet, keys are masked)
  const buildEnvSnippet = () => {
    const baseUrl = getBaseUrl()
    const pid = isProjectScoped ? projectId! : "<your-project-id>"
    return `AEGIS_BASE_URL=${baseUrl}\nAEGIS_PROJECT_ID=${pid}\nAEGIS_API_KEY=<your-api-key>`
  }

  const handleCopyEnv = async () => {
    await copyToClipboard(buildEnvSnippet())
    setCopiedEnv(true)
    setTimeout(() => setCopiedEnv(false), 2000)
  }

  const loadTokens = useCallback(() => {
    setLoading(true)

    if (isProjectScoped) {
      // Project-scoped: fetch project tokens + project info
      Promise.all([
        projectTokensApi.list(projectId),
        projectsApi.get(projectId!).catch(() => null),
      ])
        .then(([tokenData, projectData]) => {
          setTokens(tokenData || [])
          setProject(projectData)
        })
        .catch(() => { setTokens([]); setProject(null) })
        .finally(() => setLoading(false))
    } else {
      // Org-wide: fetch all tokens + all projects (for name resolution)
      Promise.all([orgTokensApi.list(), projectsApi.list()])
        .then(([tokenData, projectData]) => {
          setTokens(tokenData || [])
          setProjects(projectData || [])
        })
        .catch(() => { setTokens([]); setProjects([]) })
        .finally(() => setLoading(false))
    }
  }, [isProjectScoped, projectId])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadTokens()
  }, [loadTokens])

  const projectName = (id?: string) => {
    if (!id) return "Org-wide"
    if (isProjectScoped && project) return project.name
    const p = projects.find((proj) => proj.id === id)
    return p ? p.name : id.slice(0, 8) + "..."
  }

  // Derive counts for the filter bar (org-wide view only)
  const counts = useMemo(() => {
    const orgWide = tokens.filter((t) => !t.project_id).length
    const projectScoped = tokens.filter((t) => !!t.project_id).length
    return { all: tokens.length, orgWide, projectScoped }
  }, [tokens])

  // Projects that have tokens (for the filter dropdown, org-wide view only)
  const projectsWithTokens = useMemo(() => {
    if (isProjectScoped) return []
    const ids = new Set(tokens.filter((t) => !!t.project_id).map((t) => t.project_id!))
    return projects.filter((p) => ids.has(p.id))
  }, [tokens, projects, isProjectScoped])

  // Filter tokens (org-wide view only — project view shows all)
  const filteredTokens = useMemo(() => {
    if (isProjectScoped) return tokens
    if (scopeFilter === "all") return tokens
    if (scopeFilter === "org-wide") return tokens.filter((t) => !t.project_id)
    return tokens.filter((t) => t.project_id === scopeFilter)
  }, [tokens, scopeFilter, isProjectScoped])

  const handleRevoke = async (id: string) => {
    setRevoking(id)
    setError("")
    try {
      if (isProjectScoped) {
        await projectTokensApi.revoke(id, projectId!)
      } else {
        await orgTokensApi.revoke(id)
      }
      loadTokens()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke token")
    } finally {
      setRevoking(null)
    }
  }

  const displayName = isProjectScoped ? (project?.name || "Project") : undefined

  return (
    <div className="flex flex-col gap-4 md:gap-6">
      {/* Breadcrumb for project-scoped view */}
      {isProjectScoped && (
        <div className="flex items-center gap-3">
          <NavLink
            to={`/project/${projectId}/settings`}
            className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            Settings
          </NavLink>
          <span className="text-muted-foreground/40">/</span>
          <span className="text-sm font-medium">API Tokens</span>
        </div>
      )}

      <div>
        <h1 className="text-lg font-semibold">
          {isProjectScoped ? `API Tokens — ${displayName}` : "API Tokens"}
        </h1>
        <p className="text-sm text-muted-foreground">
          {isProjectScoped
            ? <>Manage tokens scoped to this project. These tokens can only access findings, scans, and data within <span className="font-medium text-foreground">{displayName}</span>.</>
            : "Manage API tokens for agent authentication"
          }
        </p>
      </div>

      {/* Persistent .env snippet */}
      <div className="rounded-md border border-border bg-card" id="env-snippet-card">
        <div className="flex items-center justify-between px-4 py-2 border-b border-border">
          <span className="text-xs font-medium text-muted-foreground">.env</span>
          <button
            onClick={handleCopyEnv}
            className="inline-flex items-center gap-1.5 px-2 py-1 rounded-md text-xs text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
            title="Copy .env to clipboard"
            id="copy-env-snippet-button"
          >
            {copiedEnv ? (
              <>
                <Check className="h-3 w-3 text-green-500" />
                <span className="text-green-600">Copied</span>
              </>
            ) : (
              <>
                <Copy className="h-3 w-3" />
                Copy
              </>
            )}
          </button>
        </div>
        <div className="px-4 py-3 font-mono text-xs leading-relaxed">
          <div>
            <span className="text-muted-foreground">AEGIS_BASE_URL</span>
            <span className="text-muted-foreground/60">=</span>
            <span className="text-foreground">&quot;{getBaseUrl()}&quot;</span>
          </div>
          <div>
            <span className="text-muted-foreground">AEGIS_PROJECT_ID</span>
            <span className="text-muted-foreground/60">=</span>
            <span className="text-foreground">&quot;{isProjectScoped ? projectId : "<your-project-id>"}&quot;</span>
          </div>
          <div>
            <span className="text-muted-foreground">AEGIS_API_KEY</span>
            <span className="text-muted-foreground/60">=</span>
            <span className="text-foreground">&quot;aegis_...&quot;</span>
          </div>
        </div>
      </div>

      <Card>
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Key className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-sm font-medium">
                {isProjectScoped ? "Project Tokens" : "All Tokens"}
              </CardTitle>
            </div>
            <Button size="sm" className="gap-2" onClick={() => setDialogOpen(true)} id="create-token-button">
              <Plus className="h-3.5 w-3.5" />
              Create Token
            </Button>
          </div>
          <CardDescription>
            {isProjectScoped
              ? <>Generate tokens scoped to <span className="font-medium">{displayName}</span>. Use <code className="text-xs">Authorization: Bearer aegis_xxx</code> to authenticate agents.</>
              : <>Generate tokens for agent authentication. Tokens use <code className="text-xs">Authorization: Bearer aegis_xxx</code> and are scoped to this organization.</>
            }
          </CardDescription>
        </CardHeader>
        <CardContent>
          {error && <p className="text-sm text-destructive mb-3">{error}</p>}

          {/* Scope filter bar — org-wide view only */}
          {!isProjectScoped && !loading && tokens.length > 0 && (
            <div className="flex items-center gap-2 mb-4 flex-wrap" id="token-scope-filter">
              <Filter className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <button
                onClick={() => setScopeFilter("all")}
                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
                  scopeFilter === "all"
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted/60 text-muted-foreground hover:bg-muted"
                }`}
                id="filter-all"
              >
                All
                <span className={`ml-0.5 tabular-nums ${scopeFilter === "all" ? "text-primary-foreground/70" : "text-muted-foreground/60"}`}>
                  {counts.all}
                </span>
              </button>
              <button
                onClick={() => setScopeFilter("org-wide")}
                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
                  scopeFilter === "org-wide"
                    ? "bg-primary text-primary-foreground"
                    : "bg-muted/60 text-muted-foreground hover:bg-muted"
                }`}
                id="filter-org-wide"
              >
                <Globe className="h-3 w-3" />
                Org-wide
                <span className={`ml-0.5 tabular-nums ${scopeFilter === "org-wide" ? "text-primary-foreground/70" : "text-muted-foreground/60"}`}>
                  {counts.orgWide}
                </span>
              </button>
              {projectsWithTokens.map((proj) => {
                const count = tokens.filter((t) => t.project_id === proj.id).length
                return (
                  <button
                    key={proj.id}
                    onClick={() => setScopeFilter(proj.id)}
                    className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
                      scopeFilter === proj.id
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted/60 text-muted-foreground hover:bg-muted"
                    }`}
                    id={`filter-project-${proj.slug}`}
                  >
                    <FolderKey className="h-3 w-3" />
                    {proj.name}
                    <span className={`ml-0.5 tabular-nums ${scopeFilter === proj.id ? "text-primary-foreground/70" : "text-muted-foreground/60"}`}>
                      {count}
                    </span>
                  </button>
                )
              })}
            </div>
          )}

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
              <p className="text-sm font-medium">
                {isProjectScoped ? "No project tokens" : "No API tokens"}
              </p>
              <p className="text-xs mt-1">
                {isProjectScoped
                  ? "Create a token to authenticate agents for this project."
                  : "Create a token to authenticate your scanning agents."
                }
              </p>
            </div>
          ) : filteredTokens.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              <Filter className="h-6 w-6 mx-auto mb-2 opacity-40" />
              <p className="text-sm font-medium">No tokens match this filter</p>
              <p className="text-xs mt-1">
                <button onClick={() => setScopeFilter("all")} className="text-primary hover:underline">
                  Clear filter
                </button>
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto -mx-6">
              <table className="w-full text-sm" id="tokens-table">
                <thead>
                  <tr className="border-b">
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Name</th>
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Prefix</th>
                    {/* Scope column only in org-wide view */}
                    {!isProjectScoped && (
                      <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Scope</th>
                    )}
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Expires</th>
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Status</th>
                    <th className="px-6 pb-3 text-left text-xs font-medium text-muted-foreground">Created</th>
                    <th className="px-6 pb-3 text-right text-xs font-medium text-muted-foreground"></th>
                  </tr>
                </thead>
                <tbody>
                  {filteredTokens.map((t) => {
                    const isExpired = t.expires_at && new Date(t.expires_at) < new Date()
                    const isOrgWide = !t.project_id
                    return (
                      <tr key={t.id} className="border-b last:border-0 hover:bg-muted/50 transition-colors">
                        <td className="px-6 py-3 font-medium">{t.name}</td>
                        <td className="px-6 py-3 font-mono text-xs text-muted-foreground">{t.prefix}...</td>
                        {/* Scope badge only in org-wide view */}
                        {!isProjectScoped && (
                          <td className="px-6 py-3">
                            {isOrgWide ? (
                              <Badge variant="secondary" className="text-[10px] px-1.5 py-0 gap-1">
                                <Globe className="h-2.5 w-2.5" />
                                Org-wide
                              </Badge>
                            ) : (
                              <Badge variant="outline" className="text-[10px] px-1.5 py-0 gap-1 text-blue-600 border-blue-200 bg-blue-50 dark:text-blue-400 dark:border-blue-800 dark:bg-blue-950">
                                <FolderKey className="h-2.5 w-2.5" />
                                {projectName(t.project_id)}
                              </Badge>
                            )}
                          </td>
                        )}
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
        projectId={projectId}
        projectName={displayName}
      />
    </div>
  )
}
