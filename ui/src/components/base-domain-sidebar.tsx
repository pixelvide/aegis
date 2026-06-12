import { NavLink, useLocation } from "react-router-dom"
import { useState } from "react"
import {
  User,
  Building2,
  LogOut,
  ChevronsUpDown,
  Shield,
  Lock,
  Monitor,
  Mail,
  ChevronRight,
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
  SidebarMenuSub,
  SidebarMenuSubItem,
  SidebarMenuSubButton,
  SidebarRail,
  useSidebar,
} from "@/components/ui/sidebar"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@radix-ui/react-collapsible"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useAuth } from "@/lib/auth-context"

const accountNav = [
  { to: "/orgs", icon: Building2, label: "Organizations" },
  { to: "/profile", icon: User, label: "Profile" },
  { to: "/profile/emails", icon: Mail, label: "Emails" },
]

const accessManagementNav = [
  { to: "/profile/access-management/authentication", icon: Lock, label: "Authentication" },
  { to: "/profile/access-management/sessions", icon: Monitor, label: "Active Sessions" },
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

export function BaseDomainSidebar() {
  const location = useLocation()
  const { isMobile } = useSidebar()
  const { user, logout } = useAuth()

  // Auto-expand Access Management if current path is within it
  const isAccessManagementPath = location.pathname.startsWith("/profile/access-management")
  const [accessOpen, setAccessOpen] = useState(isAccessManagementPath)

  const isActive = (to: string) => {
    if (to === "/" || to === "/profile") return location.pathname === to
    return location.pathname.startsWith(to)
  }

  const displayName = user?.name || user?.email || "User"
  const displayEmail = user?.email || ""
  const initials = userInitials(displayName)

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <NavLink to="/profile">
                <div className="flex size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                  <Shield className="h-4 w-4" />
                </div>
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-semibold">Aegis</span>
                  <span className="truncate text-xs text-muted-foreground">Account Settings</span>
                </div>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent>
        {/* Account Navigation */}
        <SidebarGroup>
          <SidebarGroupLabel>Account</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {accountNav.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <SidebarMenuButton
                    asChild
                    isActive={isActive(item.to)}
                    tooltip={item.label}
                  >
                    <NavLink to={item.to}>
                      <item.icon />
                      <span>{item.label}</span>
                    </NavLink>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        {/* Access Management — Collapsible */}
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              <Collapsible open={accessOpen} onOpenChange={setAccessOpen}>
                <SidebarMenuItem>
                  <CollapsibleTrigger asChild>
                    <SidebarMenuButton tooltip="Access Management">
                      <User />
                      <span>Access Management</span>
                      <ChevronRight className={`ml-auto h-4 w-4 transition-transform duration-200 ${accessOpen ? "rotate-90" : ""}`} />
                    </SidebarMenuButton>
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <SidebarMenuSub>
                      {accessManagementNav.map((item) => (
                        <SidebarMenuSubItem key={item.to}>
                          <SidebarMenuSubButton
                            asChild
                            isActive={isActive(item.to)}
                          >
                            <NavLink to={item.to}>
                              <span>{item.label}</span>
                            </NavLink>
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ))}
                    </SidebarMenuSub>
                  </CollapsibleContent>
                </SidebarMenuItem>
              </Collapsible>
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
                  id="base-user-menu-trigger"
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
