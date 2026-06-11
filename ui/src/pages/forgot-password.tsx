import { useState } from "react"
import { Shield, ArrowLeft, Loader2, Mail } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Link } from "react-router-dom"

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [sent, setSent] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    setLoading(true)
    try {
      const res = await fetch("/api/v1/auth/forgot-password", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email }),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({ error: "Request failed" }))
        throw new Error(data.error || "Request failed")
      }
      setSent(true)
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : "Something went wrong"
      setError(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-6 px-4">
        {/* Logo */}
        <div className="flex flex-col items-center gap-2">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary">
            <Shield className="h-6 w-6 text-primary-foreground" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">Reset Password</h1>
          <p className="text-sm text-muted-foreground text-center">
            {sent
              ? "Check your email for a reset link"
              : "Enter your email and we'll send you a reset link"}
          </p>
        </div>

        {sent ? (
          <div className="space-y-4">
            <div className="flex flex-col items-center gap-3 rounded-lg border bg-muted/30 p-6">
              <div className="flex h-12 w-12 items-center justify-center rounded-full bg-primary/10">
                <Mail className="h-6 w-6 text-primary" />
              </div>
              <p className="text-sm text-center text-muted-foreground">
                If <span className="font-medium text-foreground">{email}</span> is registered,
                you&apos;ll receive a password reset link shortly.
              </p>
            </div>
            <Link
              to="/login"
              className="flex items-center justify-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to sign in
            </Link>
          </div>
        ) : (
          <>
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <label htmlFor="reset-email" className="text-sm font-medium">
                  Email
                </label>
                <Input
                  id="reset-email"
                  type="email"
                  placeholder="you@company.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoComplete="email"
                  autoFocus
                />
              </div>

              {error && (
                <div className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {error}
                </div>
              )}

              <Button type="submit" className="w-full" disabled={loading}>
                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Send reset link
              </Button>
            </form>

            <Link
              to="/login"
              className="flex items-center justify-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeft className="h-4 w-4" />
              Back to sign in
            </Link>
          </>
        )}
      </div>
    </div>
  )
}
