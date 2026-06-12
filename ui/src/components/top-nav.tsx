import { useLocation } from "react-router-dom"
import { Search, Moon, Sun, Bell } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { SidebarTrigger } from "@/components/ui/sidebar"
import { useState, useEffect } from "react"
import { useProject } from "@/lib/project-context"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { ChevronDown, FolderKanban, Plus } from "lucide-react"
import { useNavigate } from "react-router-dom"

const routeNames: Record<string, string> = {
  "/": "Dashboard",
  "/scans": "Scans",
  "/findings": "Findings",
  "/agents": "Agents",
  "/settings": "Settings",
  "/profile": "Profile",
  "/profile/emails": "Emails",
  "/profile/access-management/authentication": "Authentication",
  "/profile/access-management/sessions": "Active Sessions",
  "/reports": "Reports",
  "/analytics": "Analytics",
}

export function TopNav() {
  const location = useLocation()
  const navigate = useNavigate()
  const { currentProject, projects, switchProject } = useProject()
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

        {/* Breadcrumb / Project Switcher */}
        <div className="flex items-center text-sm font-medium">
          {currentProject ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm" className="h-8 px-2 -ml-2 gap-1 data-[state=open]:bg-accent text-base">
                  <span className="truncate max-w-[120px] sm:max-w-[200px]">{currentProject.name}</span>
                  <ChevronDown className="size-3 text-muted-foreground" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-56">
                {projects.map((project) => (
                  <DropdownMenuItem
                    key={project.id}
                    onClick={() => switchProject(project)}
                    className="gap-2"
                  >
                    <FolderKanban className="size-4 text-muted-foreground" />
                    {project.name}
                  </DropdownMenuItem>
                ))}
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => switchProject(null)} className="text-muted-foreground">
                  View all projects
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => navigate("/projects?create=true")}>
                  <Plus className="size-4 mr-2" />
                  Create project
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : (
            <span className="truncate max-w-[120px] sm:max-w-[200px]">{currentPage}</span>
          )}
        </div>

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
