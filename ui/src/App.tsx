import { BrowserRouter, Routes, Route, useLocation } from "react-router-dom"
import { TooltipProvider } from "@/components/ui/tooltip"
import {
  SidebarProvider,
  SidebarInset,
} from "@/components/ui/sidebar"
import { Toaster } from "sonner"

import { AppSidebar } from "@/components/app-sidebar"
import { AuthProvider, useAuth } from "@/lib/auth-context"
import { OrgProvider } from "@/lib/org-context"
import { TopNav } from "@/components/top-nav"
import DashboardPage from "@/pages/dashboard"
import ScansPage from "@/pages/scans"
import FindingsPage from "@/pages/findings"
import FindingDetailPage from "@/pages/finding-detail"
import AgentsPage from "@/pages/agents"
import SettingsPage from "@/pages/settings"
import ProfilePage from "@/pages/profile"
import LoginPage from "@/pages/login"
import ForgotPasswordPage from "@/pages/forgot-password"
import ResetPasswordPage from "@/pages/reset-password"
import VerifyEmailPage from "@/pages/verify-email"

function AppLayout() {
  const { user, loading } = useAuth()
  const location = useLocation()

  // Public routes rendered without sidebar
  const publicPaths = ["/login", "/forgot-password", "/reset-password", "/verify-email"]
  if (publicPaths.some(p => location.pathname.startsWith(p))) {
    return (
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/verify-email" element={<VerifyEmailPage />} />
      </Routes>
    )
  }

  // Show nothing while checking auth
  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
      </div>
    )
  }

  // Not authenticated → login page handles redirect
  if (!user) {
    return null
  }

  return (
    <OrgProvider>
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
                      <Route path="/" element={<DashboardPage />} />
                      <Route path="/scans" element={<ScansPage />} />
                      <Route path="/findings" element={<FindingsPage />} />
                      <Route path="/findings/:id" element={<FindingDetailPage />} />
                      <Route path="/agents" element={<AgentsPage />} />
                      <Route path="/settings" element={<SettingsPage />} />
                      <Route path="/profile" element={<ProfilePage />} />
                    </Routes>
                  </div>
                </div>
              </div>
            </div>
          </SidebarInset>
        </SidebarProvider>
      </TooltipProvider>
    </OrgProvider>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/forgot-password" element={<ForgotPasswordPage />} />
          <Route path="/reset-password" element={<ResetPasswordPage />} />
          <Route path="/verify-email" element={<VerifyEmailPage />} />
          <Route path="/*" element={<AppLayout />} />
        </Routes>
        <Toaster position="bottom-right" duration={10000} closeButton />
      </AuthProvider>
    </BrowserRouter>
  )
}
