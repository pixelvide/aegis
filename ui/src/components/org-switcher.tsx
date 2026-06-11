import { ChevronsUpDown, Check, Plus, Building2 } from "lucide-react"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenuButton,
  useSidebar,
} from "@/components/ui/sidebar"
import { useOrg } from "@/lib/org-context"

export function OrgSwitcher() {
  const { isMobile } = useSidebar()
  const { orgs, currentOrg, switchOrg, loading } = useOrg()

  if (loading) {
    return (
      <SidebarMenuButton size="lg" className="animate-pulse">
        <div className="flex size-8 items-center justify-center rounded-lg bg-muted" />
        <div className="grid flex-1 gap-1">
          <div className="h-3 w-20 bg-muted rounded" />
          <div className="h-2 w-14 bg-muted rounded" />
        </div>
      </SidebarMenuButton>
    )
  }

  if (!currentOrg) {
    return (
      <SidebarMenuButton size="lg" id="org-project-switcher">
        <div className="flex size-8 items-center justify-center rounded-lg bg-muted">
          <Building2 className="h-4 w-4 text-muted-foreground" />
        </div>
        <div className="grid flex-1 text-left text-sm leading-tight">
          <span className="truncate font-semibold text-muted-foreground">No Organization</span>
          <span className="truncate text-xs text-muted-foreground">Create one to get started</span>
        </div>
      </SidebarMenuButton>
    )
  }

  const avatar = currentOrg.name.charAt(0).toUpperCase()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <SidebarMenuButton
          size="lg"
          className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
          id="org-project-switcher"
        >
          <div className="flex size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
            <span className="text-xs font-semibold">{avatar}</span>
          </div>
          <div className="grid flex-1 text-left text-sm leading-tight">
            <span className="truncate font-semibold">{currentOrg.name}</span>
            <span className="truncate text-xs text-muted-foreground capitalize">{currentOrg.plan}</span>
          </div>
          <ChevronsUpDown className="ml-auto" />
        </SidebarMenuButton>
      </DropdownMenuTrigger>

      <DropdownMenuContent
        className="w-[--radix-dropdown-menu-trigger-width] min-w-64 rounded-lg"
        align="start"
        side={isMobile ? "bottom" : "right"}
        sideOffset={4}
      >
        <DropdownMenuLabel className="text-xs text-muted-foreground flex items-center justify-between">
          <span>Organizations</span>
          <span className="font-normal tabular-nums">{orgs.length}</span>
        </DropdownMenuLabel>

        <DropdownMenuGroup>
          {orgs.map((org) => (
            <DropdownMenuItem
              key={org.id}
              onClick={() => switchOrg(org)}
              className="gap-2 p-2"
            >
              <div className="flex size-6 items-center justify-center rounded-sm border bg-background">
                <span className="text-xs font-medium">{org.name.charAt(0).toUpperCase()}</span>
              </div>
              <span className="flex-1">{org.name}</span>
              <span className="text-xs text-muted-foreground capitalize">{org.plan}</span>
              {org.id === currentOrg.id && <Check className="h-4 w-4 text-foreground" />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuGroup>

        <DropdownMenuSeparator />

        <DropdownMenuItem className="gap-2 p-2">
          <Plus className="h-4 w-4" />
          Create Organization
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
