import { useState, useEffect, useCallback } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Loader2, Smartphone, Monitor, LogOut } from "lucide-react"
import { request } from "@/lib/api"
import { toast } from "sonner"

// ─── Types ──────────────────────────────────────────────────────────────────

interface Session {
  id: string
  ip_address: string
  user_agent: string
  browser: string
  os: string
  device_type: string
  created_at: string
  last_active_at: string
  expires_at: string
  current: boolean
}

// ─── Sessions Page ──────────────────────────────────────────────────────────

export default function SessionsPage() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [loading, setLoading] = useState(true)
  const [revoking, setRevoking] = useState("")

  const fetchSessions = useCallback(async () => {
    try {
      const data = await request<{ sessions: Session[] }>("/profile/sessions")
      setSessions(data.sessions || [])
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    request<{ sessions: Session[] }>("/profile/sessions")
      .then((data) => { if (!cancelled) { setSessions(data.sessions || []); setLoading(false) } })
      .catch(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [])

  const handleRevoke = async (id: string) => {
    setRevoking(id)
    try {
      await request(`/profile/sessions/${id}`, { method: "DELETE" })
      toast.success("Session revoked")
      fetchSessions()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to revoke session")
    } finally {
      setRevoking("")
    }
  }

  const handleRevokeAll = async () => {
    try {
      await request("/profile/sessions", { method: "DELETE" })
      toast.success("All other sessions revoked")
      fetchSessions()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to revoke sessions")
    }
  }

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Active Sessions</h1>
        <p className="text-sm text-muted-foreground">
          Devices and browsers where you're currently signed in.
        </p>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle>Sessions</CardTitle>
                <CardDescription>
                  If you see a session you don't recognize, revoke it immediately and change your password.
                </CardDescription>
              </div>
              {sessions.length > 1 && (
                <Button variant="outline" size="sm" className="text-destructive" onClick={handleRevokeAll}>
                  <LogOut className="mr-2 h-4 w-4" /> Sign out all other sessions
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            {sessions.length === 0 ? (
              <p className="text-sm text-muted-foreground py-4 text-center">No active sessions found.</p>
            ) : (
              sessions.map((session) => (
                <div key={session.id} className="flex items-center justify-between rounded-lg border border-border p-4">
                  <div className="flex items-center gap-4">
                    <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-muted">
                      {session.device_type === "mobile" ? (
                        <Smartphone className="h-5 w-5 text-muted-foreground" />
                      ) : (
                        <Monitor className="h-5 w-5 text-muted-foreground" />
                      )}
                    </div>
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-semibold">
                          {session.browser || "Unknown"} · {session.os || "Unknown"}
                        </span>
                        {session.current && (
                          <Badge variant="default" className="text-xs">Current Session</Badge>
                        )}
                      </div>
                      <div className="text-xs text-muted-foreground space-y-0.5">
                        <div>IP: {session.ip_address} · {session.device_type || "desktop"}</div>
                        <div>Signed in {formatRelativeTime(session.created_at)} · Last active {formatRelativeTime(session.last_active_at)}</div>
                      </div>
                    </div>
                  </div>
                  {!session.current && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleRevoke(session.id)}
                      disabled={revoking === session.id}
                      className="text-destructive hover:text-destructive"
                    >
                      {revoking === session.id ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <LogOut className="h-4 w-4" />
                      )}
                    </Button>
                  )}
                </div>
              ))
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

// ─── Utilities ──────────────────────────────────────────────────────────────

function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMin = Math.floor(diffMs / 60000)

  if (diffMin < 1) return "just now"
  if (diffMin < 60) return `${diffMin}m ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  return `${diffDay}d ago`
}
