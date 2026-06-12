import { useEffect, useState, useCallback } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Users, UserPlus, Trash2, Mail,
} from "lucide-react"
import { membersApi } from "@/lib/api"
import { useAuth } from "@/lib/auth-context"
import type { Member, OrgRole } from "@/lib/types"

const roleBadgeColor: Record<OrgRole, string> = {
  owner: "bg-amber-500/10 text-amber-600 border-amber-200",
  admin: "bg-blue-500/10 text-blue-600 border-blue-200",
  member: "bg-emerald-500/10 text-emerald-600 border-emerald-200",
  viewer: "bg-zinc-500/10 text-zinc-600 border-zinc-200",
}

export default function MembersPage() {
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
    <div className="flex flex-col gap-4 md:gap-6">
      <div>
        <h1 className="text-lg font-semibold">Members</h1>
        <p className="text-sm text-muted-foreground">Manage your organization's team members</p>
      </div>

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
