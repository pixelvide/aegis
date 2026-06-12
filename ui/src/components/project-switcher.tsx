import { Check, ChevronsUpDown, FolderKanban, Plus } from "lucide-react"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { SidebarMenuButton } from "@/components/ui/sidebar"
import { useProject } from "@/lib/project-context"
import { useNavigate } from "react-router-dom"

export function ProjectSwitcher() {
  const { projects, currentProject, switchProject } = useProject()
  const navigate = useNavigate()

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <SidebarMenuButton
          size="lg"
          className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
        >
          <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <FolderKanban className="size-4" />
          </div>
          <div className="grid flex-1 text-left text-sm leading-tight">
            <span className="truncate font-semibold">
              {currentProject ? currentProject.name : "Select Project"}
            </span>
            <span className="truncate text-xs text-muted-foreground">
              {currentProject ? "Project" : "View all projects"}
            </span>
          </div>
          <ChevronsUpDown className="ml-auto size-4" />
        </SidebarMenuButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        className="w-[--radix-dropdown-menu-trigger-width] min-w-56 rounded-lg"
        align="start"
        sideOffset={4}
      >
        <DropdownMenuLabel className="text-xs text-muted-foreground">
          Projects
        </DropdownMenuLabel>
        {projects.map((project) => (
          <DropdownMenuItem
            key={project.id}
            onClick={() => switchProject(project)}
            className="gap-2 p-2"
          >
            <div className="flex size-6 items-center justify-center rounded-sm border">
              <FolderKanban className="size-4 shrink-0" />
            </div>
            {project.name}
            {project.id === currentProject?.id && (
              <Check className="ml-auto size-4" />
            )}
          </DropdownMenuItem>
        ))}
        {projects.length === 0 && (
          <div className="p-2 text-sm text-muted-foreground text-center">
            No projects found
          </div>
        )}
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => switchProject(null)} className="gap-2 p-2 text-muted-foreground">
          <div className="flex size-6 items-center justify-center rounded-md border bg-background">
            <FolderKanban className="size-4" />
          </div>
          <div className="font-medium text-muted-foreground">View all projects</div>
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => navigate("/projects?create=true")} className="gap-2 p-2">
          <div className="flex size-6 items-center justify-center rounded-md border bg-background">
            <Plus className="size-4" />
          </div>
          <div className="font-medium text-muted-foreground">Create project</div>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
