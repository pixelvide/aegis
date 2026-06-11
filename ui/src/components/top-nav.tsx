import { useLocation } from "react-router-dom"
import { Search, Moon, Sun, Bell } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { SidebarTrigger } from "@/components/ui/sidebar"
import { useState, useEffect } from "react"

const routeNames: Record<string, string> = {
  "/": "Dashboard",
  "/scans": "Scans",
  "/findings": "Findings",
  "/agents": "Agents",
  "/settings": "Settings",
  "/profile": "Profile",
  "/reports": "Reports",
  "/analytics": "Analytics",
}

export function TopNav() {
  const location = useLocation()
  const [dark, setDark] = useState(() => {
    if (typeof window !== "undefined") {
      return document.documentElement.classList.contains("dark")
    }
    return false
  })

  useEffect(() => {
    if (dark) {
      document.documentElement.classList.add("dark")
    } else {
      document.documentElement.classList.remove("dark")
    }
  }, [dark])

  const segments = location.pathname.split("/").filter(Boolean)
  const currentPage = routeNames[location.pathname]
    || (segments.length > 0 ? routeNames[`/${segments[0]}`] : "Dashboard")

  return (
    <header className="flex h-12 shrink-0 items-center gap-2 border-b transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-12">
      <div className="flex w-full items-center gap-1.5 px-4 lg:gap-2 lg:px-6">
        {/* Sidebar toggle — uses real shadcn SidebarTrigger */}
        <SidebarTrigger className="-ml-1" />

        <Separator orientation="vertical" className="mx-2 data-[orientation=vertical]:h-4" />

        {/* Page title */}
        <h1 className="text-base font-medium">{currentPage}</h1>

        {/* Right actions */}
        <div className="ml-auto flex items-center gap-2">
          {/* Search */}
          <Button
            variant="ghost"
            size="sm"
            className="gap-2 text-muted-foreground text-xs h-8 px-3 hidden sm:flex"
            id="search-trigger"
          >
            <Search className="h-3.5 w-3.5" />
            <span>Search...</span>
            <kbd className="inline-flex items-center gap-0.5 rounded border px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground/70">
              ⌘K
            </kbd>
          </Button>

          {/* Theme toggle */}
          <Button
            variant="ghost"
            size="icon-sm"
            className="text-muted-foreground"
            onClick={() => setDark(!dark)}
            id="theme-toggle"
          >
            {dark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </Button>

          {/* Notifications */}
          <Button
            variant="ghost"
            size="icon-sm"
            className="text-muted-foreground"
            id="notifications-button"
          >
            <Bell className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </header>
  )
}
