import { BrowserRouter, Routes, Route, useLocation } from "react-router-dom"
import { TooltipProvider } from "@/components/ui/tooltip"
import {
  SidebarProvider,
  SidebarInset,
} from "@/components/ui/sidebar"

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
import LoginPage from "@/pages/login"

function AppLayout() {
  const { user, loading } = useAuth()
  const location = useLocation()

  // Show login page without sidebar
  if (location.pathname === "/login") {
    return <LoginPage />
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
          <Route path="/*" element={<AppLayout />} />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
