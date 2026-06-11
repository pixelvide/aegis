import { createContext, useContext, useState, useEffect, useCallback } from "react"
import { useNavigate, useLocation } from "react-router-dom"

interface User {
  id: string
  email: string
  name: string
  avatar_url: string
  mfa_enabled: boolean
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
          const data = await res.json()
          setUser(data.user)
        } else {
          setUser(null)
        }
      })
      .catch(() => setUser(null))
      .finally(() => setLoading(false))
  }, [])

  // Redirect to login if not authenticated (skip public routes)
  useEffect(() => {
    const isPublic = ["/login", "/forgot-password", "/reset-password", "/verify-email"].some(
      p => location.pathname.startsWith(p)
    )
    if (!loading && !user && !isPublic) {
      navigate("/login", { replace: true })
    }
  }, [loading, user, location.pathname, navigate])

  const login = useCallback(async (email: string, password: string): Promise<LoginResult | void> => {
    const res = await fetch("/api/v1/auth/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ email, password }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({ error: "Login failed" }))
      throw new Error(data.error || "Login failed")
    }
    const data = await res.json()

    // MFA challenge — don't set user yet
    if (data.mfa_required) {
      return { mfa_required: true, mfa_token: data.mfa_token, mfa_methods: data.mfa_methods } as MFAChallenge
    }

    setUser(data.user)
    navigate("/", { replace: true })
  }, [navigate])

  const register = useCallback(async (email: string, password: string, name: string) => {
    const res = await fetch("/api/v1/auth/register", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      credentials: "include",
      body: JSON.stringify({ email, password, name }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({ error: "Registration failed" }))
      throw new Error(data.error || "Registration failed")
    }
    const data = await res.json()
    setUser(data.user)
    navigate("/", { replace: true })
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
        const data = await res.json()
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
