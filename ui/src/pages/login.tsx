import { useState } from "react"
import { Shield, Loader2, KeyRound, Mail, Smartphone, AlertCircle } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { InputOTP, InputOTPGroup, InputOTPSlot } from "@/components/ui/input-otp"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { useAuth } from "@/lib/auth-context"
import { Link } from "react-router-dom"
import { request } from "@/lib/api"
import { toast } from "sonner"

interface MFAMethod {
  id: string
  type: string
  name: string
}

export default function LoginPage() {
  const { login, register } = useAuth()
  const [mode, setMode] = useState<"login" | "register">("login")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [name, setName] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  // MFA challenge state
  const [mfaRequired, setMfaRequired] = useState(false)
  const [mfaToken, setMfaToken] = useState("")
  const [mfaMethods, setMfaMethods] = useState<MFAMethod[]>([])
  const [selectedDevice, setSelectedDevice] = useState<MFAMethod | null>(null)
  const [mfaCode, setMfaCode] = useState("")
  const [useRecovery, setUseRecovery] = useState(false)
  const [emailOtpSent, setEmailOtpSent] = useState(false)
  const [sendingOtp, setSendingOtp] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError("")
    setLoading(true)
    try {
      if (mode === "login") {
        const result = await login(email, password)
        if (result && "mfa_required" in result && result.mfa_required) {
          setMfaRequired(true)
          setMfaToken(result.mfa_token ?? "")
          const methods = result.mfa_methods ?? []
          setMfaMethods(methods)
          // Auto-select if only one method
          if (methods.length === 1) {
            setSelectedDevice(methods[0])
            // Auto-send email OTP immediately
            if (methods[0].type === "email") {
              handleSendEmailOTP(methods[0], result.mfa_token ?? "")
            }
          }
        }
      } else {
        await register(email, password, name)
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Something went wrong")
    } finally {
      setLoading(false)
    }
  }

  const handleSendEmailOTP = async (device: MFAMethod, token?: string) => {
    setSendingOtp(true)
    try {
      await request("/auth/mfa/send-email-otp", {
        method: "POST",
        body: JSON.stringify({
          mfa_token: token ?? mfaToken,
          device_id: device.id,
        }),
      })
      setEmailOtpSent(true)
      setSelectedDevice(device)
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to send code")
    } finally {
      setSendingOtp(false)
    }
  }

  const handleMFASubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    try {
      const res = await fetch("/api/v1/auth/mfa/validate", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        credentials: "include",
        body: JSON.stringify({
          mfa_token: mfaToken,
          device_id: useRecovery ? undefined : selectedDevice?.id,
          code: mfaCode,
          is_recovery: useRecovery,
        }),
      })

      if (!res.ok) {
        const data = await res.json().catch(() => ({ error: "Verification failed" }))
        throw new Error(data.error || "Verification failed")
      }

      const data = await res.json()
      if (data.recovery_codes_remaining !== undefined && data.recovery_codes_remaining < 3) {
        console.log(`Warning: only ${data.recovery_codes_remaining} recovery codes remaining`)
      }
      window.location.href = "/"
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Verification failed")
    } finally {
      setLoading(false)
    }
  }

  const resetMFA = () => {
    setMfaRequired(false)
    setMfaToken("")
    setMfaMethods([])
    setSelectedDevice(null)
    setMfaCode("")
    setUseRecovery(false)
    setEmailOtpSent(false)
    setError("")
  }

  // MFA challenge screen
  if (mfaRequired) {
    // Device selector — shown when multiple methods and none selected yet
    if (!selectedDevice && !useRecovery && mfaMethods.length > 1) {
      return (
        <div className="flex min-h-screen items-center justify-center bg-background">
          <div className="w-full max-w-sm space-y-6 px-4">
            <div className="flex flex-col items-center gap-2">
              <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary">
                <KeyRound className="h-6 w-6 text-primary-foreground" />
              </div>
              <h1 className="text-2xl font-semibold tracking-tight">Choose verification method</h1>
              <p className="text-sm text-muted-foreground text-center">
                Select how you'd like to verify your identity
              </p>
            </div>

            <div className="space-y-2">
              {mfaMethods.map((method) => (
                <button
                  key={method.id}
                  className="w-full flex items-center gap-3 rounded-lg border border-border p-3 text-left hover:bg-accent transition-colors"
                  onClick={() => {
                    if (method.type === "email") {
                      handleSendEmailOTP(method)
                    } else {
                      setSelectedDevice(method)
                    }
                  }}
                  disabled={sendingOtp}
                >
                  {method.type === "totp" ? (
                    <Smartphone className="h-5 w-5 text-muted-foreground" />
                  ) : (
                    <Mail className="h-5 w-5 text-muted-foreground" />
                  )}
                  <div>
                    <div className="text-sm font-medium">{method.name}</div>
                    <div className="text-xs text-muted-foreground">
                      {method.type === "totp" ? "Authenticator app" : "Email verification code"}
                    </div>
                  </div>
                </button>
              ))}
            </div>



            <div className="text-center text-sm text-muted-foreground">
              <button
                type="button"
                className="font-medium text-primary underline-offset-4 hover:underline"
                onClick={() => { setUseRecovery(true); setError("") }}
              >
                Use a recovery code
              </button>
            </div>

            <div className="text-center">
              <button
                type="button"
                className="text-sm text-muted-foreground hover:text-foreground transition-colors"
                onClick={resetMFA}
              >
                ← Back to sign in
              </button>
            </div>
          </div>
        </div>
      )
    }

    // Code entry screen
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="w-full max-w-sm space-y-6 px-4">
          <div className="flex flex-col items-center gap-2">
            <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary">
              <KeyRound className="h-6 w-6 text-primary-foreground" />
            </div>
            <h1 className="text-2xl font-semibold tracking-tight">Two-Factor Authentication</h1>
            <p className="text-sm text-muted-foreground text-center">
              {useRecovery
                ? "Enter one of your recovery codes"
                : selectedDevice?.type === "email"
                  ? `Enter the code sent to ${selectedDevice.name}`
                  : `Enter the code from "${selectedDevice?.name ?? "Authenticator"}"`}
            </p>
          </div>

          <form onSubmit={handleMFASubmit} className="space-y-4">
            <div className="space-y-2 text-center">
              <label htmlFor="mfa-code" className="text-sm font-medium">
                {useRecovery ? "Recovery Code" : "Verification Code"}
              </label>
              {useRecovery ? (
                <Input
                  id="mfa-code"
                  type="text"
                  placeholder="abcd1234"
                  value={mfaCode}
                  onChange={(e) => setMfaCode(e.target.value)}
                  required
                  autoComplete="one-time-code"
                  autoFocus
                  className="text-center text-lg tracking-widest font-mono"
                  maxLength={8}
                />
              ) : (
                <div className="flex justify-center">
                  <InputOTP
                    maxLength={6}
                    value={mfaCode}
                    onChange={(value) => setMfaCode(value)}
                    autoFocus
                  >
                    <InputOTPGroup>
                      <InputOTPSlot index={0} />
                      <InputOTPSlot index={1} />
                      <InputOTPSlot index={2} />
                      <InputOTPSlot index={3} />
                      <InputOTPSlot index={4} />
                      <InputOTPSlot index={5} />
                    </InputOTPGroup>
                  </InputOTP>
                </div>
              )}
            </div>



            <Button type="submit" className="w-full" disabled={loading}>
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Verify
            </Button>
          </form>

          {selectedDevice?.type === "email" && (
            <div className="text-center text-sm text-muted-foreground">
              <button
                type="button"
                className="font-medium text-primary underline-offset-4 hover:underline"
                onClick={() => handleSendEmailOTP(selectedDevice)}
                disabled={sendingOtp}
              >
                {sendingOtp ? "Sending..." : emailOtpSent ? "Resend code" : "Send code"}
              </button>
            </div>
          )}

          <div className="text-center text-sm text-muted-foreground">
            <button
              type="button"
              className="font-medium text-primary underline-offset-4 hover:underline"
              onClick={() => {
                setUseRecovery(!useRecovery)
                setMfaCode("")
                setError("")
                if (!useRecovery) setSelectedDevice(null)
              }}
            >
              {useRecovery ? "Use authenticator instead" : "Use a recovery code"}
            </button>
          </div>

          <div className="text-center">
            <button
              type="button"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
              onClick={() => {
                if (mfaMethods.length > 1 && !useRecovery) {
                  setSelectedDevice(null)
                  setMfaCode("")
                  setError("")
                } else {
                  resetMFA()
                }
              }}
            >
              {mfaMethods.length > 1 && !useRecovery ? "← Choose another method" : "← Back to sign in"}
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="w-full max-w-sm space-y-6 px-4">
        {/* Logo */}
        <div className="flex flex-col items-center gap-2">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary">
            <Shield className="h-6 w-6 text-primary-foreground" />
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">Aegis</h1>
          <p className="text-sm text-muted-foreground">
            {mode === "login"
              ? "Sign in to your account"
              : "Create a new account"}
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          {mode === "register" && (
            <div className="space-y-2">
              <label htmlFor="name" className="text-sm font-medium">
                Name
              </label>
              <Input
                id="name"
                type="text"
                placeholder="Your name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                autoComplete="name"
              />
            </div>
          )}

          <div className="space-y-2">
            <label htmlFor="email" className="text-sm font-medium">
              Email
            </label>
            <Input
              id="email"
              type="email"
              placeholder="you@company.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="email"
            />
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <label htmlFor="password" className="text-sm font-medium">
                Password
              </label>
              {mode === "login" && (
                <Link
                  to="/forgot-password"
                  className="text-xs text-muted-foreground hover:text-primary transition-colors"
                  tabIndex={-1}
                >
                  Forgot password?
                </Link>
              )}
            </div>
            <Input
              id="password"
              type="password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              autoComplete={mode === "login" ? "current-password" : "new-password"}
            />
            {mode === "register" && (
              <p className="text-xs text-muted-foreground">
                Must be 8+ characters with uppercase, lowercase, and a digit
              </p>
            )}
          </div>

          <Button type="submit" className="w-full" disabled={loading}>
            {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {mode === "login" ? "Sign in" : "Create account"}
          </Button>
        </form>

        {/* Toggle */}
        <div className="text-center text-sm text-muted-foreground">
          {mode === "login" ? (
            <>
              Don&apos;t have an account?{" "}
              <button
                type="button"
                className="font-medium text-primary underline-offset-4 hover:underline"
                onClick={() => { setMode("register"); setError("") }}
              >
                Sign up
              </button>
            </>
          ) : (
            <>
              Already have an account?{" "}
              <button
                type="button"
                className="font-medium text-primary underline-offset-4 hover:underline"
                onClick={() => { setMode("login"); setError("") }}
              >
                Sign in
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
