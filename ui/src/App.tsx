import { useEffect, useCallback, useRef } from "react"
import { BrowserRouter, Routes, Route, useLocation } from "react-router-dom"
import { TooltipProvider } from "@/components/ui/tooltip"
import {
  SidebarProvider,
  SidebarInset,
} from "@/components/ui/sidebar"
import { Toaster } from "sonner"

import { AppSidebar } from "@/components/app-sidebar"
import { BaseDomainSidebar } from "@/components/base-domain-sidebar"
import { AuthProvider, useAuth } from "@/lib/auth-context"
import { OrgProvider, useOrg } from "@/lib/org-context"
import { ProjectProvider } from "@/lib/project-context"
import { useDomainMode, baseDomainUrl } from "@/lib/domain"
import { TopNav } from "@/components/top-nav"
import DashboardPage from "@/pages/dashboard"
import ScansPage from "@/pages/scans"
import FindingsPage from "@/pages/findings"
import FindingDetailPage from "@/pages/finding-detail"
import AgentsPage from "@/pages/agents"
import SettingsPage from "@/pages/settings"
import MembersPage from "@/pages/members"
import ApiTokensPage from "@/pages/settings/api-tokens"
import FeaturesPage from "@/pages/settings/features"
import ProfileOverviewPage from "@/pages/profile"
import EmailsPage from "@/pages/profile/emails"
import AuthenticationPage from "@/pages/profile/authentication"
import SessionsPage from "@/pages/profile/sessions"
import ProjectsPage from "@/pages/projects"
import OrgsPage from "@/pages/orgs"
import LoginPage from "@/pages/login"
import ForgotPasswordPage from "@/pages/forgot-password"
import ResetPasswordPage from "@/pages/reset-password"
import VerifyEmailPage from "@/pages/verify-email"
import VerifyEmailPendingPage from "@/pages/verify-email-pending"

// Auth-related paths that live on the base domain only
const AUTH_PATHS = ["/login", "/forgot-password", "/reset-password", "/verify-email", "/verify-email-pending"]

// Base domain paths — pages that live on the identity domain (not org subdomain)
const BASE_DOMAIN_PATHS = [...AUTH_PATHS, "/profile", "/orgs"]

// ─── Subdomain Auth Redirect ──────────────────────────────────────────────
// When on an org subdomain and the user hits an auth page or base-domain page,
// redirect to the base domain.
function SubdomainAuthGuard() {
  const location = useLocation()
  const { mode, baseDomain } = useDomainMode()

  useEffect(() => {
    if (mode !== "org") return

    const isBaseDomainPage = BASE_DOMAIN_PATHS.some(p => location.pathname.startsWith(p))
    if (!isBaseDomainPage) return

    const port = window.location.port ? `:${window.location.port}` : ""
    const returnTo = `${window.location.protocol}//${window.location.hostname}${port}/`

    const isAuthPage = AUTH_PATHS.some(p => location.pathname.startsWith(p))
    const targetPath = location.pathname + location.search

    if (isAuthPage) {
      // Auth pages: include return_to so login redirects back
      const separator = targetPath.includes("?") ? "&" : "?"
      window.location.href = baseDomainUrl(baseDomain, `${targetPath}${separator}return_to=${encodeURIComponent(returnTo)}`)
    } else {
      // Profile, orgs pages: just redirect, no return_to needed
      window.location.href = baseDomainUrl(baseDomain, targetPath)
    }
  }, [mode, baseDomain, location.pathname, location.search])

  return null
}

// ─── Base Domain Redirect ─────────────────────────────────────────────────
// When on the base domain root "/" and the user is authenticated + verified,
// redirect to their org subdomain. The base domain only serves auth + identity pages.
function BaseDomainGuard() {
  const { user, loading: authLoading } = useAuth()
  const { mode, baseDomain, loading: domainLoading } = useDomainMode()
  const location = useLocation()
  const redirectingRef = useRef(false)

  const redirectToOrgSubdomain = useCallback(async () => {
    if (redirectingRef.current) return
    redirectingRef.current = true
    try {
      const res = await fetch("/api/v1/orgs", {
        credentials: "include",
        headers: { "Content-Type": "application/json" },
      })
      if (res.ok) {
        const body = await res.json()
        const data = body.result ?? body
        const orgs = data.orgs || []
        if (orgs.length > 0) {
          const savedSlug = localStorage.getItem("aegis_current_org_slug")
          const org = orgs.find((o: { slug: string }) => o.slug === savedSlug) || orgs[0]
          const protocol = window.location.protocol
          const port = window.location.port ? `:${window.location.port}` : ""
          window.location.href = `${protocol}//${org.slug}.${baseDomain}${port}/`
          return
        }
      }
    } catch {
      // Fall through — show org picker
    }
    // No orgs found → send to org picker to create one
    redirectingRef.current = false
  }, [baseDomain])

  useEffect(() => {
    if (authLoading || domainLoading) return
    if (mode !== "base") return

    // Only auto-redirect on root "/" — other base domain pages render normally
    const isBaseDomainPage = BASE_DOMAIN_PATHS.some(p => location.pathname.startsWith(p))
    if (isBaseDomainPage) return

    // Root path: authenticated + verified → redirect to org subdomain
    if (location.pathname === "/" && user && user.email_verified) {
      redirectToOrgSubdomain()
    }
  }, [authLoading, domainLoading, mode, user, location.pathname, redirectToOrgSubdomain])

  return null
}

// ─── Base Domain Layout ───────────────────────────────────────────────────
// Identity layout for base domain pages (profile, orgs) — uses BaseDomainSidebar.
function BaseDomainLayout() {
  const { user, loading: authLoading } = useAuth()
  const { loading: domainLoading } = useDomainMode()
  const location = useLocation()

  if (authLoading || domainLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  if (!user) return null // auth-context will redirect to /login

  // Determine which page to render based on path
  let page = null
  if (location.pathname === "/profile/access-management/authentication") {
    page = <AuthenticationPage />
  } else if (location.pathname === "/profile/access-management/sessions") {
    page = <SessionsPage />
  } else if (location.pathname === "/profile/emails") {
    page = <EmailsPage />
  } else if (location.pathname.startsWith("/profile")) {
    page = <ProfileOverviewPage />
  } else if (location.pathname.startsWith("/orgs")) {
    page = <OrgsPage />
  }

  return (
    <TooltipProvider>
      <SidebarProvider>
        <BaseDomainSidebar />
        <SidebarInset>
          <div className="flex flex-1 flex-col">
            <div className="@container/main flex flex-1 flex-col gap-2">
              <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
                <div className="px-4 lg:px-6">
                  {page}
                </div>
              </div>
            </div>
          </div>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}

// ─── Org Access Guard ─────────────────────────────────────────────────────
// Wraps org workspace content. If the user doesn't have access to the
// subdomain org, shows an access denied page instead of the workspace.
function OrgAccessGuard({ children }: { children: React.ReactNode }) {
  const { accessDenied, loading, baseDomain } = useOrg()

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  if (accessDenied) {
    const slug = baseDomain
      ? window.location.hostname.slice(0, -(baseDomain.length + 1))
      : window.location.hostname
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center space-y-4 max-w-md px-4">
          <div className="text-5xl">🔒</div>
          <h1 className="text-xl font-semibold">Access Denied</h1>
          <p className="text-sm text-muted-foreground">
            You don't have access to the organization <span className="font-mono font-medium text-foreground">{slug}</span>.
            Contact the organization admin to request an invitation.
          </p>
          {baseDomain && (
            <a
              href={baseDomainUrl(baseDomain, "/orgs")}
              className="inline-flex items-center justify-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 transition-colors"
            >
              Go to My Organizations
            </a>
          )}
        </div>
      </div>
    )
  }

  return <>{children}</>
}

// ─── App Layout (org subdomain or header-only mode) ───────────────────────
function AppLayout() {
  const { user, loading: authLoading } = useAuth()
  const { mode, loading: domainLoading } = useDomainMode()
  const location = useLocation()

  // Auth/base-domain pages rendered without sidebar
  // (shouldn't happen on org subdomain — SubdomainAuthGuard redirects — but handle gracefully)
  if (BASE_DOMAIN_PATHS.some(p => location.pathname.startsWith(p))) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/verify-email" element={<VerifyEmailPage />} />
        <Route path="/verify-email-pending" element={<VerifyEmailPendingPage />} />
        <Route path="/profile" element={<ProfileOverviewPage />} />
        <Route path="/profile/emails" element={<EmailsPage />} />
        <Route path="/profile/access-management/authentication" element={<AuthenticationPage />} />
        <Route path="/profile/access-management/sessions" element={<SessionsPage />} />
        <Route path="/orgs" element={<OrgsPage />} />
      </Routes>
    )
  }

  // Show loading spinner
  if (authLoading || domainLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  // On the base domain root, show redirect spinner
  // (BaseDomainGuard handles the actual redirect)
  if (mode === "base") {
    if (!user) return null
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center space-y-3">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent mx-auto" />
          <p className="text-sm text-muted-foreground">Redirecting to your organization...</p>
        </div>
      </div>
    )
  }

  // Not authenticated
  if (!user) return null

  return (
    <OrgProvider>
      <OrgAccessGuard>
        <ProjectProvider>
          <TooltipProvider>
            <SidebarProvider>
              <AppSidebar />
              <SidebarInset>
                <TopNav />
                <div className="flex flex-1 flex-col">
                  <div className="@container/main flex flex-1 flex-col gap-2">
                    <div className="flex flex-col gap-4 py-4 md:gap-6 md:py-6">
                      <div className="px-4 lg:px-6">
                        <Routes>
                          <Route path="/" element={<ProjectsPage />} />
                          <Route path="/members" element={<MembersPage />} />
                          <Route path="/settings" element={<SettingsPage />} />
                          <Route path="/settings/api-tokens" element={<ApiTokensPage />} />
                          <Route path="/settings/features" element={<FeaturesPage />} />

                          <Route path="/project/:projectId">
                            <Route path="dashboard" element={<DashboardPage />} />
                            <Route path="scans" element={<ScansPage />} />
                            <Route path="findings" element={<FindingsPage />} />
                            <Route path="findings/:id" element={<FindingDetailPage />} />
                            <Route path="agents" element={<AgentsPage />} />
                            <Route path="settings" element={<SettingsPage />} />
                            <Route path="settings/api-tokens" element={<ApiTokensPage />} />
                          </Route>
                        </Routes>
                      </div>
                    </div>
                  </div>
                </div>
              </SidebarInset>
            </SidebarProvider>
          </TooltipProvider>
        </ProjectProvider>
      </OrgAccessGuard>
    </OrgProvider>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <SubdomainAuthGuard />
        <BaseDomainGuard />
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/forgot-password" element={<ForgotPasswordPage />} />
          <Route path="/reset-password" element={<ResetPasswordPage />} />
          <Route path="/verify-email" element={<VerifyEmailPage />} />
          <Route path="/verify-email-pending" element={<VerifyEmailPendingPage />} />
          <Route path="/profile" element={<BaseDomainLayout />} />
          <Route path="/profile/emails" element={<BaseDomainLayout />} />
          <Route path="/profile/access-management/authentication" element={<BaseDomainLayout />} />
          <Route path="/profile/access-management/sessions" element={<BaseDomainLayout />} />
          <Route path="/orgs" element={<BaseDomainLayout />} />
          <Route path="/*" element={<AppLayout />} />
        </Routes>
        <Toaster position="bottom-right" duration={10000} closeButton />
      </AuthProvider>
    </BrowserRouter>
  )
}
