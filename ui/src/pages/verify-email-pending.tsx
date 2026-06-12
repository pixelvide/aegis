import { useState } from "react"
import { Mail, Loader2, Shield, CheckCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { useAuth } from "@/lib/auth-context"
import { toast } from "sonner"

export default function VerifyEmailPendingPage() {
  const { user, logout, refreshUser } = useAuth()
  const [resending, setResending] = useState(false)
  const [sent, setSent] = useState(false)
  const [checking, setChecking] = useState(false)

  const handleResend = async () => {
    setResending(true)
    try {
      // Get the user's emails to find the primary unverified one
      const emailsRes = await fetch("/api/v1/profile/emails", {
        credentials: "include",
      })
      if (!emailsRes.ok) throw new Error("Failed to load emails")
      const emailsBody = await emailsRes.json()
      const emails = emailsBody.result ?? emailsBody
      const primaryEmail = (Array.isArray(emails) ? emails : []).find(
        (e: { is_primary: boolean; verified: boolean }) => e.is_primary && !e.verified
      )
      if (!primaryEmail) {
        // Email may already be verified — refresh user state
        await refreshUser()
        return
      }

      // Send verification email
      const res = await fetch(`/api/v1/profile/emails/${primaryEmail.id}/send-verification`, {
        method: "POST",
        credentials: "include",
      })
      if (!res.ok) throw new Error("Failed to send verification email")
      setSent(true)
      toast.success("Verification email sent!")
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to send verification email")
    } finally {
      setResending(false)
    }
  }

  const handleCheckStatus = async () => {
    setChecking(true)
    try {
      await refreshUser()
      // If email is now verified, the auth context redirect effect will
      // automatically navigate away from this page
    } catch {
      // ignore
    } finally {
      setChecking(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-6 px-4">
        <div className="flex flex-col items-center gap-2">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary">
            <Shield className="h-6 w-6 text-primary-foreground" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">Verify your email</h1>
          <p className="text-sm text-muted-foreground text-center">
            We sent a verification email to{" "}
            <span className="font-medium text-foreground">{user?.email}</span>.
            Check your inbox and click the link to continue.
          </p>
        </div>

        <div className="flex flex-col items-center gap-3 rounded-lg border bg-muted/30 p-6">
          <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10">
            <Mail className="h-6 w-6 text-primary" />
          </div>
          <p className="text-sm text-center text-muted-foreground">
            {sent
              ? "A new verification email has been sent. Check your spam folder if you don't see it."
              : "Didn't receive the email? Check your spam folder or resend it."}
          </p>
        </div>

        <div className="space-y-3">
          <Button
            variant="outline"
            className="w-full"
            onClick={handleResend}
            disabled={resending}
          >
            {resending ? (
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            ) : sent ? (
              <CheckCircle className="mr-2 h-4 w-4" />
            ) : (
              <Mail className="mr-2 h-4 w-4" />
            )}
            {sent ? "Resend again" : "Resend verification email"}
          </Button>

          <Button
            variant="default"
            className="w-full"
            onClick={handleCheckStatus}
            disabled={checking}
          >
            {checking && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            I've verified my email
          </Button>
        </div>

        <div className="text-center">
          <button
            type="button"
            className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            onClick={logout}
          >
            Sign out
          </button>
        </div>
      </div>
    </div>
  )
}
