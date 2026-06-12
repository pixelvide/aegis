import { createContext, useContext, useState, useEffect, useCallback } from "react"
import { useNavigate, useLocation } from "react-router-dom"
import type { OrgsListResponse } from "@/lib/api"

interface User {
  id: string
  email: string
  name: string
  avatar_url: string
  mfa_enabled: boolean
  email_verified: boolean
}

interface MFAMethod {
  id: string
  type: string
  name: string
}

interface MFAChallenge {
  mfa_required: true
  mfa_token: string
  mfa_methods: MFAMethod[]
}

interface LoginResult {
  mfa_required?: boolean
  mfa_token?: string
  mfa_methods?: MFAMethod[]
}

interface AuthContextValue {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<LoginResult | void>
  register: (email: string, password: string, name: string) => Promise<void>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  loading: true,
  login: async () => {},
  register: async () => {},
  logout: async () => {},
  refreshUser: async () => {},
})

export function useAuth() {
  return useContext(AuthContext)
}

// getValidReturnTo extracts and validates the ?return_to= query parameter.
// Only allows URLs that share the same base domain to prevent open redirect attacks.
// Returns the validated URL string, or null if invalid/missing.
function getValidReturnTo(): string | null {
  const params = new URLSearchParams(window.location.search)
  const returnTo = params.get("return_to")
  if (!returnTo) return null

  try {
    const url = new URL(returnTo)
    const currentHost = window.location.hostname

    // Allow same host or subdomains of the current host
    // e.g., if we're on aegis.io, allow acme.aegis.io
    if (url.hostname === currentHost || url.hostname.endsWith("." + currentHost)) {
      // Only allow http/https protocols
      if (url.protocol === "http:" || url.protocol === "https:") {
        return returnTo
      }
    }
  } catch {
    // Invalid URL — ignore
  }

  return null
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()
  const location = useLocation()

  // Check session on mount
  useEffect(() => {
    fetch("/api/v1/auth/me", { credentials: "include" })
      .then(async (res) => {
        if (res.ok) {
          const body = await res.json()
          const data = body.result ?? body
          setUser(data.user)
        } else {
          setUser(null)
        }
      })
      .catch(() => setUser(null))
      .finally(() => setLoading(false))
  }, [])

  // Redirect to login if not authenticated, or to verify-email-pending if unverified
  useEffect(() => {
    // Paths that don't require a redirect to /login
    const allowedPaths = ["/login", "/forgot-password", "/reset-password", "/verify-email", "/verify-email-pending"]
    const isAllowed = allowedPaths.some(p => location.pathname.startsWith(p))
    if (!loading && !user && !isAllowed) {
      navigate("/login", { replace: true })
    }
    // If authenticated but email not verified, redirect to pending page
    // (allow /verify-email so the verification callback works)
    const verifyAllowed = ["/verify-email-pending", "/verify-email"]
    if (!loading && user && !user.email_verified && !verifyAllowed.some(p => location.pathname.startsWith(p))) {
      navigate("/verify-email-pending", { replace: true })
    }
  }, [loading, user, location.pathname, navigate])

  // Redirect to the user's first org subdomain after auth.
  // When base_domain is set (production), navigates to {slug}.{base_domain}.
  // In dev mode (no base_domain), just navigates to /.
  const redirectToOrg = useCallback(async () => {
    try {
      const res = await fetch("/api/v1/orgs", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      })
      if (res.ok) {
        const body: { result?: OrgsListResponse } = await res.json()
        const data = (body.result ?? body) as OrgsListResponse
        const baseDomain = data.base_domain
        const orgs = data.orgs || []

        if (baseDomain && orgs.length > 0) {
          // Pick the saved org or fall back to the first one
          const savedSlug = localStorage.getItem("aegis_current_org_slug")
          const org = orgs.find(o => o.slug === savedSlug) || orgs[0]
          const protocol = window.location.protocol
          const port = window.location.port ? `:${window.location.port}` : ""
          window.location.href = `${protocol}//${org.slug}.${baseDomain}${port}/`
          return
        }
      }
    } catch {
      // Fall through to default navigation
    }
    navigate("/", { replace: true })
  }, [navigate])

  const login = useCallback(async (email: string, password: string): Promise<LoginResult | void> => {
    const res = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ email, password }),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({ errors: [{ message: "Login failed" }] }))
      const msg = body.errors?.[0]?.message || body.error || "Login failed"
      throw new Error(msg)
    }
    const body = await res.json()
    const data = body.result ?? body

    // MFA challenge — don't set user yet
    if (data.mfa_required) {
      return { mfa_required: true, mfa_token: data.mfa_token, mfa_methods: data.mfa_methods } as MFAChallenge
    }

    setUser(data.user)

    // If email not verified, go to verification pending page
    if (!data.user.email_verified) {
      navigate("/verify-email-pending", { replace: true })
      return
    }

    // If there's a validated return_to param, redirect back to that subdomain
    const returnTo = getValidReturnTo()
    if (returnTo) {
      window.location.href = returnTo
    } else {
      await redirectToOrg()
    }
  }, [redirectToOrg, navigate])

  const register = useCallback(async (email: string, password: string, name: string) => {
    const res = await fetch("/api/v1/auth/register", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ email, password, name }),
    })
    if (!res.ok) {
      const body = await res.json().catch(() => ({ errors: [{ message: "Registration failed" }] }))
      const msg = body.errors?.[0]?.message || body.error || "Registration failed"
      throw new Error(msg)
    }
    const body = await res.json()
    const data = body.result ?? body
    setUser(data.user)

    // After registration, always go to verification pending page
    navigate("/verify-email-pending", { replace: true })
  }, [navigate])

  const logout = useCallback(async () => {
    await fetch("/api/v1/auth/logout", {
      method: "POST",
      credentials: "include",
    })
    setUser(null)
    navigate("/login", { replace: true })
  }, [navigate])

  const refreshUser = useCallback(async () => {
    try {
      const res = await fetch("/api/v1/auth/me", { credentials: "include" })
      if (res.ok) {
        const body = await res.json()
        const data = body.result ?? body
        setUser(data.user)
      }
    } catch {
      // ignore
    }
  }, [])

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout, refreshUser }}>
      {children}
    </AuthContext.Provider>
  )
}
