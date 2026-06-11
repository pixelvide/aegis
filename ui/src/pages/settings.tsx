import { useEffect, useState, useCallback } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  Save, Server, Shield, Database, Globe, Users, UserPlus, Trash2, Mail, Key, Plus,
} from "lucide-react"
import { membersApi, tokensApi, projectsApi } from "@/lib/api"
import { useAuth } from "@/lib/auth-context"
import type { Member, OrgRole, APIToken, Project } from "@/lib/types"
import { formatDate } from "@/lib/utils"
import { CreateTokenDialog } from "@/components/create-token-dialog"

type Tab = "general" | "members" | "tokens"

const roleBadgeColor: Record<OrgRole, string> = {
  owner: "bg-amber-500/10 text-amber-600 border-amber-200",
  admin: "bg-blue-500/10 text-blue-600 border-blue-200",
  member: "bg-emerald-500/10 text-emerald-600 border-emerald-200",
  viewer: "bg-zinc-500/10 text-zinc-600 border-zinc-200",
}

function MembersTab() {
  const { user } = useAuth()
  const [members, setMembers] = useState<Member[]>([])
  const [loading, setLoading] = useState(true)
  const [inviteEmail, setInviteEmail] = useState("")
  const [inviteRole, setInviteRole] = useState<OrgRole>("member")
  const [inviting, setInviting] = useState(false)
  const [error, setError] = useState("")

  const loadMembers = useCallback(() => {
    membersApi
      .list()
      .then((data) => { setMembers(data); setLoading(false) })
      .catch(() => { setMembers([]); setLoading(false) })
  }, [])

  useEffect(() => { loadMembers() }, [loadMembers])

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!inviteEmail.trim()) return

    setInviting(true)
    setError("")
    try {
      await membersApi.invite(inviteEmail.trim(), inviteRole)
      setInviteEmail("")
      loadMembers()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to invite")
    } finally {
      setInviting(false)
    }
  }

  const handleRemove = async (userId: string) => {
    if (!confirm("Remove this member from the organization?")) return
    try {
      await membersApi.remove(userId)
      loadMembers()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove member")
    }
  }

  return (
    <div className="space-y-6">
      {/* Invite */}
      <Card>
        <CardHeader className="pb-4">
          <div className="flex items-center gap-2">
            <UserPlus className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-sm font-medium">Invite Member</CardTitle>
          </div>
          <CardDescription>Add a team member by email address</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleInvite} className="flex gap-3 items-end">
            <div className="flex-1">
              <label className="text-sm font-medium block mb-1.5">Email</label>
              <Input
                type="email"
                placeholder="colleague@company.com"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                id="invite-email-input"
              />
            </div>
            <div className="w-32">
              <label className="text-sm font-medium block mb-1.5">Role</label>
              <select
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value as OrgRole)}
                id="invite-role-select"
              >
                <option value="member">Member</option>
                <option value="admin">Admin</option>
                <option value="viewer">Viewer</option>
              </select>
            </div>
            <Button type="submit" disabled={inviting || !inviteEmail.trim()} className="gap-2">
              <Mail className="h-3.5 w-3.5" />
              {inviting ? "Inviting..." : "Invite"}
            </Button>
          </form>
          {error && <p className="text-sm text-destructive mt-2">{error}</p>}
        </CardContent>
      </Card>

      {/* Member list */}
      <Card>
        <CardHeader className="pb-4">
          <div className="flex items-center gap-2">
            <Users className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-sm font-medium">
              Members {!loading && <span className="text-muted-foreground font-normal">({members.length})</span>}
            </CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <div key={i} className="flex items-center gap-3">
                  <div className="h-9 w-9 rounded-full bg-muted animate-pulse" />
                  <div className="flex-1 space-y-1.5">
                    <div className="h-4 bg-muted rounded w-32 animate-pulse" />
                    <div className="h-3 bg-muted rounded w-48 animate-pulse" />
                  </div>
                </div>
              ))}
            </div>
          ) : members.length === 0 ? (
            <p className="text-sm text-muted-foreground">No members yet.</p>
          ) : (
            <div className="space-y-1">
              {members.map((m) => (
                <div
                  key={m.user_id}
                  className="flex items-center gap-3 py-2.5 px-2 rounded-md hover:bg-muted/50 transition-colors group"
                >
                  <div className="flex size-9 items-center justify-center rounded-full bg-muted shrink-0">
                    <span className="text-xs font-medium text-muted-foreground">
                      {(m.name || m.email)[0].toUpperCase()}
                    </span>
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium truncate">{m.name || "—"}</span>
                      <Badge
                        variant="outline"
                        className={`text-[10px] px-1.5 py-0 ${roleBadgeColor[m.role]}`}
                      >
                        {m.role}
                      </Badge>
                    </div>
                    <p className="text-xs text-muted-foreground truncate">{m.email}</p>
                  </div>
                  {m.user_id !== user?.id && m.role !== "owner" && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-destructive"
                      onClick={() => handleRemove(m.user_id)}
                      title="Remove member"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function TokensTab() {
  const [tokens, setTokens] = useState<APIToken[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [revoking, setRevoking] = useState<string | null>(null)
  const [error, setError] = useState("")

  const loadTokens = useCallback(() => {
    setLoading(true)
    Promise.all([tokensApi.list(), projectsApi.list()])
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
      await tokensApi.revoke(id)
      loadTokens()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke token")
    } finally {
      setRevoking(null)
    }
  }

  return (
    <div className="space-y-6">
      {/* Create */}
      <Card>
        <CardHeader className="pb-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Key className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-sm font-medium">API Tokens</CardTitle>
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

export default function SettingsPage() {
  const [tab, setTab] = useState<Tab>("general")

  return (
    <div className="flex flex-col gap-4 md:gap-6">
      {/* Tab switcher */}
      <div className="flex gap-1 border-b">
        {([
          { key: "general", label: "General", icon: Settings2Icon },
          { key: "members", label: "Members", icon: Users },
          { key: "tokens", label: "API Tokens", icon: Key },
        ] as const).map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={`flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-px ${
              tab === t.key
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            <t.icon className="h-4 w-4" />
            {t.label}
          </button>
        ))}
      </div>

      {tab === "general" ? (
        <div className="grid gap-4 md:gap-6 lg:grid-cols-2">
          {/* Agent Configuration */}
          <Card>
            <CardHeader className="pb-4">
              <div className="flex items-center gap-2">
                <Shield className="h-4 w-4 text-muted-foreground" />
                <CardTitle className="text-sm font-medium">Agent Configuration</CardTitle>
              </div>
              <CardDescription>Configure how agents are launched</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="text-sm font-medium block mb-1.5">Agent Binary Path</label>
                <Input type="text" defaultValue="aegis" id="agent-binary-input" />
                <p className="text-xs text-muted-foreground mt-1.5">
                  Path to the aegis binary or command name in PATH
                </p>
              </div>
              <div>
                <label className="text-sm font-medium block mb-1.5">Docker Image</label>
                <Input type="text" defaultValue="aegis-security:latest" id="docker-image-input" />
                <p className="text-xs text-muted-foreground mt-1.5">Image used for sandboxed scans</p>
              </div>
              <div>
                <label className="text-sm font-medium block mb-1.5">Default Persona</label>
                <select
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
                  id="default-persona-select"
                >
                  <option value="sharingan">Sharingan — Deep analysis</option>
                  <option value="senku">Senku — Scientific approach</option>
                  <option value="killua">Killua — Speed-focused</option>
                </select>
              </div>
              <Separator />
              <Button size="sm" className="gap-2">
                <Save className="h-3.5 w-3.5" />
                Save Configuration
              </Button>
            </CardContent>
          </Card>

          {/* Server Info */}
          <Card>
            <CardHeader className="pb-4">
              <div className="flex items-center gap-2">
                <Server className="h-4 w-4 text-muted-foreground" />
                <CardTitle className="text-sm font-medium">Server</CardTitle>
              </div>
              <CardDescription>Server and environment information</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between py-1">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Globe className="h-3.5 w-3.5" />
                  API Endpoint
                </div>
                <Badge variant="secondary" className="font-mono text-xs">
                  {window.location.host}
                </Badge>
              </div>
              <Separator />
              <div className="flex items-center justify-between py-1">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Database className="h-3.5 w-3.5" />
                  Database
                </div>
                <Badge variant="secondary" className="font-mono text-xs">PostgreSQL 16</Badge>
              </div>
              <Separator />
              <div className="flex items-center justify-between py-1">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Shield className="h-3.5 w-3.5" />
                  Version
                </div>
                <Badge variant="secondary" className="font-mono text-xs">0.1.0</Badge>
              </div>
            </CardContent>
          </Card>
        </div>
      ) : tab === "members" ? (
        <MembersTab />
      ) : (
        <TokensTab />
      )}
    </div>
  )
}

// Simple settings icon (re-using Shield but labeled differently)
function Settings2Icon(props: React.SVGProps<SVGSVGElement>) {
  return <Shield {...props} />
}
