import { NavLink, useLocation, useNavigate } from "react-router-dom"
import {
  LayoutDashboard,
  Search,
  Bug,
  Bot,
  Settings,
  LogOut,
  User,
  ChevronsUpDown,
  FolderKanban,
  Users,
  Key,
  ToggleLeft,
  ArrowLeft,
} from "lucide-react"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "@/components/ui/sidebar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { OrgSwitcher } from "@/components/org-switcher"
import { useAuth } from "@/lib/auth-context"
import { useProject } from "@/lib/project-context"
import { useDomainMode, baseDomainUrl } from "@/lib/domain"

const orgNav = [
  { to: "/", icon: FolderKanban, label: "Projects" },
  { to: "/members", icon: Users, label: "Members" },
  { to: "/settings", icon: Settings, label: "Settings" },
  { to: "/settings/api-tokens", icon: Key, label: "API Tokens" },
  { to: "/settings/features", icon: ToggleLeft, label: "Features" },
]

const projectNav = [
  { to: "dashboard", icon: LayoutDashboard, label: "Dashboard" },
  { to: "scans", icon: Search, label: "Scans" },
  { to: "findings", icon: Bug, label: "Findings" },
  { to: "agents", icon: Bot, label: "Agents" },
]


const projectBottomNav = [
  { to: "settings", icon: Settings, label: "Settings" },
  { to: "settings/api-tokens", icon: Key, label: "API Tokens" },
]

function userInitials(name: string): string {
  return name
    .split(" ")
    .map((w) => w[0])
    .filter(Boolean)
    .slice(0, 2)
    .join("")
    .toUpperCase()
}

export function AppSidebar() {
  const location = useLocation()
  const { isMobile } = useSidebar()
  const { user, logout } = useAuth()
  const { currentProject } = useProject()
  const { baseDomain } = useDomainMode()
  const navigate = useNavigate()

  const isActive = (to: string) => {
    if (to === "/") return location.pathname === "/"
    if (currentProject) {
      const fullPath = `/project/${currentProject.id}/${to}`
      return location.pathname.startsWith(fullPath)
    }
    // Exact match for org-level routes to avoid prefix collisions
    // (e.g., /settings shouldn't match /settings/api-tokens)
    return location.pathname === to
  }

  const getLink = (to: string) => {
    if (to === "/" || !currentProject) return to
    return `/project/${currentProject.id}/${to}`
  }

  const displayName = user?.name || user?.email || "User"
  const displayEmail = user?.email || ""
  const initials = userInitials(displayName)

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <OrgSwitcher />
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        {/* Back to org-level navigation */}
        {currentProject && (
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem>
                <SidebarMenuButton
                  tooltip="All Projects"
                  onClick={() => navigate("/")}
                  className="text-muted-foreground hover:text-foreground"
                  id="back-to-org-nav"
                >
                  <ArrowLeft />
                  <span>All Projects</span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
        )}


        {/* Main Navigation */}
        <SidebarGroup>
          {currentProject && <SidebarGroupLabel>Project View</SidebarGroupLabel>}
          <SidebarGroupContent>
            <SidebarMenu>
              {(currentProject ? projectNav : orgNav).map((item) => (
                <SidebarMenuItem key={item.to}>
                  <SidebarMenuButton
                    asChild
                    isActive={isActive(item.to)}
                    tooltip={item.label}
                  >
                    <NavLink to={getLink(item.to)}>
                      <item.icon />
                      <span>{item.label}</span>
                    </NavLink>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>


        {/* Bottom group — pushed to bottom */}
        <SidebarGroup className="mt-auto">
          <SidebarGroupContent>
            <SidebarMenu>
              {currentProject && projectBottomNav.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <SidebarMenuButton asChild isActive={isActive(item.to)} tooltip={item.label}>
                    <NavLink to={getLink(item.to)}>
                      <item.icon />
                      <span>{item.label}</span>
                    </NavLink>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}

            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      {/* Footer: User with DropdownMenu */}
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  size="lg"
                  className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                  id="user-menu-trigger"
                >
                  <div className="flex size-8 items-center justify-center rounded-lg bg-muted grayscale">
                    <span className="text-xs font-medium text-muted-foreground">{initials}</span>
                  </div>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-medium">{displayName}</span>
                    <span className="truncate text-xs text-muted-foreground">{displayEmail}</span>
                  </div>
                  <ChevronsUpDown className="ml-auto size-4" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>

              <DropdownMenuContent
                className="w-[--radix-dropdown-menu-trigger-width] min-w-56 rounded-lg"
                side={isMobile ? "bottom" : "right"}
                align="end"
                sideOffset={4}
              >
                <div className="flex items-center gap-2 px-1 py-1.5 text-left text-sm">
                  <div className="flex size-8 items-center justify-center rounded-lg bg-muted">
                    <span className="text-xs font-medium text-muted-foreground">{initials}</span>
                  </div>
                  <div className="grid flex-1 text-left text-sm leading-tight">
                    <span className="truncate font-semibold">{displayName}</span>
                    <span className="truncate text-xs text-muted-foreground">{displayEmail}</span>
                  </div>
                </div>

                <DropdownMenuSeparator />

                <DropdownMenuGroup>
                  <DropdownMenuItem onClick={() => {
                    if (baseDomain) {
                      window.location.href = baseDomainUrl(baseDomain, "/profile")
                    } else {
                      window.location.href = "/profile"
                    }
                  }}>
                    <User className="mr-2 h-4 w-4" />
                    Manage Account
                  </DropdownMenuItem>
                </DropdownMenuGroup>

                <DropdownMenuSeparator />

                <DropdownMenuItem onClick={logout}>
                  <LogOut className="mr-2 h-4 w-4" />
                  Log out
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  )
}

