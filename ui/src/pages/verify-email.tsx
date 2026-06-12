import { useState, useEffect } from "react"
import { Loader2, CheckCircle, XCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Link, useSearchParams } from "react-router-dom"

export default function VerifyEmailPage() {
  const [searchParams] = useSearchParams()
  const token = searchParams.get("token") || ""
  const emailId = searchParams.get("email_id") || ""

  const [status, setStatus] = useState<"loading" | "success" | "error" | "invalid">(
    token && emailId ? "loading" : "invalid"
  )
  const [error, setError] = useState("")

  useEffect(() => {
    if (!token || !emailId) return

    fetch("/api/v1/auth/verify-email", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ token, email_id: emailId }),
    })
      .then(async (res) => {
        if (res.ok) {
          setStatus("success")
        } else {
          const body = await res.json().catch(() => ({ errors: [{ message: "Verification failed" }] }))
          setError(body.errors?.[0]?.message || body.error || "Verification failed")
          setStatus("error")
        }
      })
      .catch(() => {
        setError("Network error")
        setStatus("error")
      })
  }, [token, emailId])

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-6 px-4">
        <div className="flex flex-col items-center gap-2">
          <div className={`flex h-12 w-12 items-center justify-center rounded-xl ${
            status === "success" ? "bg-emerald-500/10" : status === "error" || status === "invalid" ? "bg-destructive/10" : "bg-primary"
          }`}>
            {status === "loading" ? (
              <Loader2 className="h-6 w-6 text-primary-foreground animate-spin" />
            ) : status === "success" ? (
              <CheckCircle className="h-6 w-6 text-emerald-500" />
            ) : (
              <XCircle className="h-6 w-6 text-destructive" />
            )}
          </div>

          <h1 className="text-2xl font-semibold tracking-tight">
            {status === "loading" && "Verifying..."}
            {status === "success" && "Email Verified!"}
            {status === "error" && "Verification Failed"}
            {status === "invalid" && "Invalid Link"}
          </h1>

          <p className="text-sm text-muted-foreground text-center">
            {status === "loading" && "Please wait while we verify your email address"}
            {status === "success" && "Your email address has been verified successfully"}
            {status === "error" && (error || "The verification link is invalid or has expired")}
            {status === "invalid" && "This verification link is missing or invalid"}
          </p>
        </div>

        {status === "success" && (
          <Link to="/">
            <Button className="w-full">Go to Dashboard</Button>
          </Link>
        )}

        {(status === "error" || status === "invalid") && (
          <div className="space-y-3">
            <Link to="/profile">
              <Button variant="outline" className="w-full">
                Go to Profile — Resend verification
              </Button>
            </Link>
            <Link to="/">
              <Button variant="ghost" className="w-full">
                Go to Dashboard
              </Button>
            </Link>
          </div>
        )}
      </div>
    </div>
  )
}
