import { useState, useEffect, useCallback } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { InputOTP, InputOTPGroup, InputOTPSlot } from "@/components/ui/input-otp"
import { Alert, AlertDescription } from "@/components/ui/alert"
import {
  Lock, Loader2, Copy, Check, ShieldCheck, ShieldOff,
  Mail, Smartphone, RefreshCw, Eye, EyeOff, KeyRound, AlertTriangle, Trash2,
} from "lucide-react"
import { useAuth } from "@/lib/auth-context"
import { request } from "@/lib/api"
import { QRCodeSVG } from "qrcode.react"
import { toast } from "sonner"
import { PasswordModal } from "@/components/password-modal"

// ─── Types ──────────────────────────────────────────────────────────────────

interface UserEmail {
  id: string
  email: string
  is_primary: boolean
  verified: boolean
  verified_at?: string
  created_at: string
}

interface MFADevice {
  id: string
  name: string
  type: string
  email?: string
  verified: boolean
  verified_at?: string
  last_used_at?: string
  created_at: string
}

// ─── Tab Constants ──────────────────────────────────────────────────────────

type AuthTab = "two-factor" | "password"

const AUTH_TABS: { id: AuthTab; label: string; icon: React.ReactNode }[] = [
  { id: "two-factor", label: "Two Factor Authentication", icon: <ShieldCheck className="h-4 w-4" /> },
  { id: "password", label: "Password", icon: <KeyRound className="h-4 w-4" /> },
]

// ─── Authentication Page ────────────────────────────────────────────────────

export default function AuthenticationPage() {
  const [activeTab, setActiveTab] = useState<AuthTab>("two-factor")

  return (
    <div className="space-y-6">
      {/* Page header */}
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Authentication</h1>
        <p className="text-sm text-muted-foreground">
          Manage how you sign in to your account.
        </p>
      </div>

      {/* Tabs */}
      <div className="flex border-b border-border">
        {AUTH_TABS.map((tab) => (
          <button
            key={tab.id}
            className={`flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab.id
                ? "border-primary text-primary"
                : "border-transparent text-muted-foreground hover:text-foreground hover:border-border"
            }`}
            onClick={() => setActiveTab(tab.id)}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === "two-factor" && <TwoFactorTab />}
      {activeTab === "password" && <PasswordTab />}
    </div>
  )
}

// ─── Password Tab ───────────────────────────────────────────────────────────

function PasswordTab() {
  const [currentPassword, setCurrentPassword] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    if (newPassword !== confirmPassword) {
      toast.error("Passwords do not match")
      return
    }

    setLoading(true)
    try {
      await request("/auth/change-password", {
        method: "POST",
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      })
      toast.success("Password changed successfully")
      setCurrentPassword("")
      setNewPassword("")
      setConfirmPassword("")
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to change password")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>Change Password</CardTitle>
          <CardDescription>
            Update your password. You'll need to enter your current password to confirm.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4 max-w-md">
            <div className="space-y-2">
              <label className="text-sm font-medium">Current Password</label>
              <Input type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">New Password</label>
              <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required minLength={8} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Confirm New Password</label>
              <Input type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} required />
            </div>

            <Button type="submit" disabled={loading}>
              {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              <Lock className="mr-2 h-4 w-4" />
              Update Password
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

// ─── Two Factor Tab ─────────────────────────────────────────────────────────

function TwoFactorTab() {
  const { user, refreshUser } = useAuth()
  const [devices, setDevices] = useState<MFADevice[]>([])
  const [loading, setLoading] = useState(true)
  const [recoveryCount, setRecoveryCount] = useState(0)

  // TOTP setup state
  const [showTotpSetup, setShowTotpSetup] = useState(false)
  const [totpName, setTotpName] = useState("")
  const [totpSecret, setTotpSecret] = useState("")
  const [totpUrl, setTotpUrl] = useState("")
  const [totpDeviceId, setTotpDeviceId] = useState("")
  const [totpCode, setTotpCode] = useState("")
  const [totpVerifying, setTotpVerifying] = useState(false)

  // Recovery codes display
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([])
  const [showRecoveryCodes, setShowRecoveryCodes] = useState(false)
  const [copiedCodes, setCopiedCodes] = useState(false)

  // Enable/disable state
  const [actionLoading, setActionLoading] = useState(false)

  // Email MFA state
  const [emails, setEmails] = useState<UserEmail[]>([])

  // Password modal state
  const [passwordModal, setPasswordModal] = useState<{
    open: boolean
    title: string
    description: string
    onConfirm: (password: string) => void
    loading?: boolean
    error?: string
  }>({ open: false, title: "", description: "", onConfirm: () => {} })

  const fetchDevices = useCallback(async () => {
    try {
      const data = await request<{ devices: MFADevice[] }>("/profile/mfa/devices")
      setDevices(data.devices || [])
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  const fetchRecoveryCount = useCallback(async () => {
    try {
      const data = await request<{ remaining: number }>("/profile/mfa/recovery-codes")
      setRecoveryCount(data.remaining ?? 0)
    } catch {
      // ignore
    }
  }, [])

  // Data fetching on mount
  useEffect(() => {
    let cancelled = false
    Promise.all([
      request<{ devices: MFADevice[] }>("/profile/mfa/devices"),
      request<{ remaining: number }>("/profile/mfa/recovery-codes"),
      request<{ emails: UserEmail[] }>("/profile/emails"),
    ]).then(([devData, recData, emData]) => {
      if (cancelled) return
      setDevices(devData.devices || [])
      setRecoveryCount(recData.remaining ?? 0)
      setEmails((emData.emails || []).filter((e: UserEmail) => e.verified))
      setLoading(false)
    }).catch(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [])

  // Add TOTP device
  const handleAddTotp = async () => {
    try {
      const data = await request<{ secret: string; url: string; device: MFADevice }>("/profile/mfa/devices/totp", {
        method: "POST",
        body: JSON.stringify({ name: totpName || "Authenticator" }),
      })
      setTotpSecret(data.secret)
      setTotpUrl(data.url)
      setTotpDeviceId(data.device?.id || "")
      setShowTotpSetup(true)
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to create TOTP device")
    }
  }

  // Verify TOTP device
  const handleVerifyTotp = async (e: React.FormEvent) => {
    e.preventDefault()
    setTotpVerifying(true)
    try {
      await request(`/profile/mfa/devices/totp/${totpDeviceId}/verify`, {
        method: "POST",
        body: JSON.stringify({ code: totpCode }),
      })
      setShowTotpSetup(false)
      setTotpCode("")
      setTotpSecret("")
      setTotpUrl("")
      setTotpName("")
      toast.success("TOTP device verified")
      fetchDevices()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Invalid code")
    } finally {
      setTotpVerifying(false)
    }
  }

  // Add email MFA device
  const handleAddEmailDevice = async (emailId: string) => {
    try {
      await request("/profile/mfa/devices/email", {
        method: "POST",
        body: JSON.stringify({ email_id: emailId }),
      })
      toast.success("Email MFA device added")
      fetchDevices()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to add email device")
    }
  }

  // Remove device (uses password modal)
  const handleRemoveDevice = (deviceId: string) => {
    setPasswordModal({
      open: true,
      title: "Remove MFA Device",
      description: "Enter your password to confirm device removal.",
      onConfirm: async (password: string) => {
        setPasswordModal(prev => ({ ...prev, loading: true, error: undefined }))
        try {
          const data = await request<{ mfa_auto_disabled?: boolean }>(`/profile/mfa/devices/${deviceId}`, {
            method: "DELETE",
            body: JSON.stringify({ password }),
          })
          setPasswordModal({ open: false, title: "", description: "", onConfirm: () => {} })
          if (data.mfa_auto_disabled) {
            toast.warning("Device removed. MFA has been automatically disabled — no verified devices remain.")
          } else {
            toast.success("Device removed")
          }
          fetchDevices()
          refreshUser()
        } catch (err: unknown) {
          setPasswordModal(prev => ({
            ...prev,
            loading: false,
            error: err instanceof Error ? err.message : "Failed to remove device",
          }))
        }
      },
    })
  }

  // Enable MFA
  const handleEnableMFA = async () => {
    setActionLoading(true)
    try {
      const data = await request<{ recovery_codes: string[] }>("/profile/mfa/enable", { method: "POST" })
      setRecoveryCodes(data.recovery_codes || [])
      setShowRecoveryCodes(true)
      toast.success("MFA has been enabled!")
      fetchDevices()
      fetchRecoveryCount()
      refreshUser()
    } catch (err: unknown) {
      toast.error(err instanceof Error ? err.message : "Failed to enable MFA")
    } finally {
      setActionLoading(false)
    }
  }

  // Disable MFA (uses password modal)
  const handleDisableMFA = () => {
    setPasswordModal({
      open: true,
      title: "Disable Two-Factor Authentication",
      description: "Enter your password to disable MFA. All MFA devices and recovery codes will be removed.",
      onConfirm: async (password: string) => {
        setPasswordModal(prev => ({ ...prev, loading: true, error: undefined }))
        try {
          await request("/profile/mfa/disable", {
            method: "POST",
            body: JSON.stringify({ password }),
          })
          setPasswordModal({ open: false, title: "", description: "", onConfirm: () => {} })
          toast.success("MFA has been disabled")
          fetchDevices()
          fetchRecoveryCount()
          refreshUser()
        } catch (err: unknown) {
          setPasswordModal(prev => ({
            ...prev,
            loading: false,
            error: err instanceof Error ? err.message : "Failed to disable MFA",
          }))
        }
      },
    })
  }

  // Regenerate recovery codes (uses password modal)
  const handleRegenCodes = () => {
    setPasswordModal({
      open: true,
      title: "Regenerate Recovery Codes",
      description: "Enter your password to regenerate recovery codes. Your old codes will be invalidated.",
      onConfirm: async (password: string) => {
        setPasswordModal(prev => ({ ...prev, loading: true, error: undefined }))
        try {
          const data = await request<{ recovery_codes: string[] }>("/profile/mfa/recovery-codes/regenerate", {
            method: "POST",
            body: JSON.stringify({ password }),
          })
          setPasswordModal({ open: false, title: "", description: "", onConfirm: () => {} })
          setRecoveryCodes(data.recovery_codes || [])
          setShowRecoveryCodes(true)
          fetchRecoveryCount()
        } catch (err: unknown) {
          setPasswordModal(prev => ({
            ...prev,
            loading: false,
            error: err instanceof Error ? err.message : "Failed to regenerate codes",
          }))
        }
      },
    })
  }

  const copyRecoveryCodes = () => {
    navigator.clipboard.writeText(recoveryCodes.join("\n"))
    setCopiedCodes(true)
    setTimeout(() => setCopiedCodes(false), 2000)
  }

  if (loading) {
    return <div className="flex items-center justify-center py-12"><Loader2 className="h-6 w-6 animate-spin text-muted-foreground" /></div>
  }

  const verifiedDevices = devices.filter(d => d.verified)
  const mfaEnabled = user?.mfa_enabled ?? false

  return (
    <>
    <div className="space-y-6">
      {/* MFA Status */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                {mfaEnabled ? <ShieldCheck className="h-5 w-5 text-green-600" /> : <ShieldOff className="h-5 w-5 text-muted-foreground" />}
                Two-Factor Authentication
              </CardTitle>
              <CardDescription className="mt-1">
                {mfaEnabled
                  ? "MFA is enabled — a second factor is required when signing in."
                  : "MFA is disabled. Add a device and enable MFA to secure your account."}
              </CardDescription>
            </div>
            <div>
              {mfaEnabled ? (
                <Button variant="outline" className="text-destructive" onClick={handleDisableMFA}>
                  <ShieldOff className="mr-2 h-4 w-4" /> Disable MFA
                </Button>
              ) : (
                <Button
                  onClick={handleEnableMFA}
                  disabled={verifiedDevices.length === 0 || actionLoading}
                >
                  {actionLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  <ShieldCheck className="mr-2 h-4 w-4" /> Enable MFA
                </Button>
              )}
            </div>
          </div>
        </CardHeader>

      </Card>

      {/* Recovery codes display (shown after enable/regenerate) */}
      {showRecoveryCodes && recoveryCodes.length > 0 && (
        <Card className="border-yellow-300 dark:border-yellow-700">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <KeyRound className="h-5 w-5 text-yellow-600" />
              Recovery Codes
            </CardTitle>
            <CardDescription>
              Save these codes in a safe place. Each code can only be used once. If you lose your MFA device, use a recovery code to sign in.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-2 gap-2 mb-4 max-w-sm">
              {recoveryCodes.map((code, i) => (
                <code key={i} className="rounded bg-muted px-3 py-2 text-sm font-mono text-center">{code}</code>
              ))}
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={copyRecoveryCodes}>
                {copiedCodes ? <Check className="mr-2 h-4 w-4" /> : <Copy className="mr-2 h-4 w-4" />}
                {copiedCodes ? "Copied!" : "Copy all"}
              </Button>
              <Button variant="ghost" size="sm" onClick={() => { setShowRecoveryCodes(false); setRecoveryCodes([]) }}>
                <EyeOff className="mr-2 h-4 w-4" /> Hide
              </Button>
            </div>
          </CardContent>
        </Card>
      )}


      {/* MFA Devices */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle>MFA Devices</CardTitle>
              <CardDescription>Authenticator apps and email OTP methods.</CardDescription>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={handleAddTotp}>
                <Smartphone className="mr-2 h-4 w-4" /> Add TOTP
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-3">
          {devices.length === 0 && !showTotpSetup ? (
            <p className="text-sm text-muted-foreground py-4 text-center">No MFA devices configured yet.</p>
          ) : (
            devices.map((device) => (
              <div key={device.id} className="flex items-center justify-between rounded-lg border border-border p-3">
                <div className="flex items-center gap-3">
                  {device.type === "totp" ? (
                    <Smartphone className="h-4 w-4 text-muted-foreground" />
                  ) : (
                    <Mail className="h-4 w-4 text-muted-foreground" />
                  )}
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium">{device.name || device.type.toUpperCase()}</span>
                      <Badge variant="outline" className="text-xs">{device.type}</Badge>
                      {device.verified ? (
                        <Badge variant="secondary" className="text-xs text-green-600">Verified</Badge>
                      ) : (
                        <Badge variant="outline" className="text-xs text-yellow-600">Pending</Badge>
                      )}
                    </div>
                    {device.last_used_at && (
                      <span className="text-xs text-muted-foreground">Last used: {new Date(device.last_used_at).toLocaleDateString()}</span>
                    )}
                  </div>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => handleRemoveDevice(device.id)}
                  className="text-destructive hover:text-destructive"
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>
            ))
          )}

          {/* Add Email OTP device */}
          {emails.length > 0 && (
            <div className="pt-2">
              <p className="text-xs text-muted-foreground mb-2">Add email as MFA method:</p>
              <div className="flex flex-wrap gap-2">
                {emails
                  .filter(e => !devices.some(d => d.type === "email" && d.email === e.email))
                  .map((email) => (
                    <Button
                      key={email.id}
                      variant="outline"
                      size="sm"
                      onClick={() => handleAddEmailDevice(email.id)}
                    >
                      <Mail className="mr-2 h-3 w-3" /> {email.email}
                    </Button>
                  ))}
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* TOTP Setup Modal/Card */}
      {showTotpSetup && (
        <Card className="border-primary">
          <CardHeader>
            <CardTitle>Set Up TOTP</CardTitle>
            <CardDescription>Scan the QR code with your authenticator app (Google Authenticator, Authy, etc.), then enter the 6-digit code.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="space-y-4 max-w-md">
              <div className="space-y-2">
                <label className="text-sm font-medium">Device Name</label>
                <Input value={totpName} onChange={(e) => setTotpName(e.target.value)} placeholder="e.g., Work Phone" />
              </div>

              {/* QR Code */}
              {totpUrl && (
                <div className="flex flex-col items-center gap-3 py-2">
                  <div className="rounded-lg border bg-white p-3">
                    <QRCodeSVG value={totpUrl} size={180} level="M" />
                  </div>
                  <p className="text-xs text-muted-foreground">Scan this with your authenticator app</p>
                </div>
              )}

              {/* Manual secret key fallback */}
              <div className="space-y-2">
                <label className="text-sm font-medium">Or enter this key manually</label>
                <div className="flex items-center gap-2">
                  <code className="flex-1 rounded bg-muted px-3 py-2 text-sm font-mono break-all">{totpSecret}</code>
                  <Button variant="outline" size="sm" onClick={() => navigator.clipboard.writeText(totpSecret)}>
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </div>

              <form onSubmit={handleVerifyTotp} className="space-y-3">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Verification Code</label>
                  <InputOTP
                    maxLength={6}
                    value={totpCode}
                    onChange={(value) => setTotpCode(value)}
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
                <div className="flex gap-2">
                  <Button type="submit" disabled={totpVerifying}>
                    {totpVerifying && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Verify & Add
                  </Button>
                  <Button variant="ghost" type="button" onClick={() => { setShowTotpSetup(false); setTotpCode(""); setTotpSecret("") }}>
                    Cancel
                  </Button>
                </div>
              </form>
            </div>
          </CardContent>
        </Card>
      )}

      {/* Recovery Codes */}
      {mfaEnabled && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <KeyRound className="h-5 w-5" />
                  Recovery Codes
                </CardTitle>
                <CardDescription>{recoveryCount} of 8 codes remaining</CardDescription>
              </div>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" onClick={handleRegenCodes}>
                  <RefreshCw className="mr-2 h-4 w-4" /> Regenerate
                </Button>
                {recoveryCodes.length > 0 && !showRecoveryCodes && (
                  <Button variant="outline" size="sm" onClick={() => setShowRecoveryCodes(true)}>
                    <Eye className="mr-2 h-4 w-4" /> Show
                  </Button>
                )}
              </div>
            </div>
          </CardHeader>
          {recoveryCount < 3 && (
            <CardContent>
              <Alert className="border-amber-500/50 text-amber-700 dark:text-amber-400 [&>svg]:text-amber-600">
                <AlertTriangle className="h-4 w-4" />
                <AlertDescription>
                  You have fewer than 3 recovery codes remaining. Consider regenerating them.
                </AlertDescription>
              </Alert>
            </CardContent>
          )}
        </Card>
      )}
    </div>

      {/* Password Confirmation Modal */}
      <PasswordModal
        open={passwordModal.open}
        title={passwordModal.title}
        description={passwordModal.description}
        onConfirm={passwordModal.onConfirm}
        onCancel={() => setPasswordModal({ open: false, title: "", description: "", onConfirm: () => {} })}
        loading={passwordModal.loading}
        error={passwordModal.error}
      />
    </>
  )
}
