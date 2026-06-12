import { useState, useEffect, useCallback } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Loader2, Mail, Plus, Star, Trash2, Send } from "lucide-react"
import { request } from "@/lib/api"
import { toast } from "sonner"

// ─── Types ──────────────────────────────────────────────────────────────────

interface UserEmail {
  id: string
  email: string
  is_primary: boolean
  verified: boolean
  verified_at?: string
  created_at: string
}

// ─── Emails Page ────────────────────────────────────────────────────────────

export default function EmailsPage() {
  const [emails, setEmails] = useState<UserEmail[]>([])
  const [loading, setLoading] = useState(true)
  const [newEmail, setNewEmail] = useState("")
  const [adding, setAdding] = useState(false)
  const [showAddForm, setShowAddForm] = useState(false)

  const fetchEmails = useCallback(async () => {
    try {
      const data = await request<{ emails: UserEmail[] }>("/profile/emails")
      setEmails(data.emails || [])
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    request<{ emails: UserEmail[] }>("/profile/emails")
      .then((data) => { if (!cancelled) { setEmails(data.emails || []); setLoading(false) } })
      .catch(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [])

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    setAdding(true)
    try {
      await request("/profile/emails", {
        method: "POST",
        body: JSON.stringify({ email: newEmail }),
      })
      setNewEmail("")
      setShowAddForm(false)
      toast.success("Email added. Check your inbox for a verification link.")
      fetchEmails()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to add email")
    } finally {
      setAdding(false)
    }
  }

  const handleRemove = async (id: string) => {
    try {
      await request(`/profile/emails/${id}`, { method: "DELETE" })
      fetchEmails()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to remove email")
    }
  }

  const handleSetPrimary = async (id: string) => {
    try {
      await request(`/profile/emails/${id}/set-primary`, { method: "POST" })
      toast.success("Primary email updated")
      fetchEmails()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to set primary")
    }
  }

  const handleSendVerification = async (id: string) => {
    try {
      await request(`/profile/emails/${id}/send-verification`, { method: "POST" })
      toast.success("Verification email sent")
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to send verification")
    }
  }

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Emails</h1>
        <p className="text-sm text-muted-foreground">
          Manage your email addresses. Your primary email is used for login and notifications.
        </p>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      ) : (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <div>
              <CardTitle>Email Addresses</CardTitle>
              <CardDescription>
                Add or remove email addresses associated with your account.
              </CardDescription>
            </div>
            {!showAddForm && (
              <Button variant="outline" size="sm" onClick={() => setShowAddForm(true)}>
                <Plus className="mr-2 h-4 w-4" />
                Add new email
              </Button>
            )}
          </CardHeader>
          <CardContent className="space-y-3">
            {/* Inline add form */}
            {showAddForm && (
              <div className="rounded-lg border border-border p-4 space-y-3">
                <h4 className="text-sm font-semibold">Add new email</h4>
                <form onSubmit={handleAdd} className="space-y-3">
                  <div>
                    <label className="text-sm font-medium text-muted-foreground">Email address</label>
                    <Input
                      type="email"
                      placeholder="name@example.com"
                      value={newEmail}
                      onChange={(e) => setNewEmail(e.target.value)}
                      required
                      autoFocus
                      className="mt-1"
                    />
                  </div>
                  <div className="flex items-center gap-2">
                    <Button type="submit" size="sm" disabled={adding}>
                      {adding && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                      Add email address
                    </Button>
                    <Button type="button" variant="outline" size="sm" onClick={() => { setShowAddForm(false); setNewEmail("") }}>
                      Cancel
                    </Button>
                  </div>
                </form>
              </div>
            )}

            {/* Email list */}
            {emails.map((email) => (
              <div key={email.id} className="flex items-center justify-between rounded-lg border border-border p-3">
                <div className="flex items-center gap-3">
                  <Mail className="h-4 w-4 text-muted-foreground" />
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{email.email}</span>
                      {email.is_primary && (
                        <Badge variant="default" className="text-xs">
                          <Star className="mr-1 h-3 w-3" /> Primary
                        </Badge>
                      )}
                      {email.verified ? (
                        <Badge variant="secondary" className="text-xs text-green-600">Verified</Badge>
                      ) : (
                        <Badge variant="outline" className="text-xs text-yellow-600">Unverified</Badge>
                      )}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  {!email.verified && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleSendVerification(email.id)}
                      title="Send verification email"
                    >
                      <Send className="h-4 w-4" />
                    </Button>
                  )}
                  {!email.is_primary && email.verified && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleSetPrimary(email.id)}
                      title="Set as primary"
                    >
                      <Star className="h-4 w-4" />
                    </Button>
                  )}
                  {!email.is_primary && (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleRemove(email.id)}
                      title="Remove email"
                      className="text-destructive hover:text-destructive"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
