import { useEffect, useState, useCallback } from "react"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { FolderKanban, Plus, Calendar } from "lucide-react"
import { projectsApi } from "@/lib/api"
import { formatDate } from "@/lib/utils"
import type { Project } from "@/lib/types"

export default function ProjectsPage() {
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [newName, setNewName] = useState("")
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState("")

  const loadProjects = useCallback(() => {
    projectsApi
      .list()
      .then(setProjects)
      .catch(() => setProjects([]))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    loadProjects()
  }, [loadProjects])

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return

    setCreating(true)
    setError("")
    try {
      await projectsApi.create({ name: newName.trim() })
      setNewName("")
      setDialogOpen(false)
      loadProjects()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create project")
    } finally {
      setCreating(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Projects</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Organize findings and scope agent tokens by project.
          </p>
        </div>
        <Button size="sm" className="gap-2" onClick={() => setDialogOpen(true)} id="create-project-button">
          <Plus className="h-3.5 w-3.5" />
          Create Project
        </Button>
      </div>

      {loading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <CardContent className="pt-6 space-y-3">
                <div className="h-5 bg-muted rounded w-32 animate-pulse" />
                <div className="h-4 bg-muted rounded w-24 animate-pulse" />
                <div className="h-3 bg-muted rounded w-40 animate-pulse" />
              </CardContent>
            </Card>
          ))}
        </div>
      ) : projects.length === 0 ? (
        <Card>
          <CardContent className="py-16">
            <div className="text-center text-muted-foreground">
              <FolderKanban className="h-12 w-12 mx-auto mb-4 opacity-40" />
              <p className="font-medium text-lg text-foreground">No projects yet</p>
              <p className="text-sm mt-1 mb-4">
                Create a project to organize your findings and scope agent tokens.
              </p>
              <Button size="sm" className="gap-2" onClick={() => setDialogOpen(true)}>
                <Plus className="h-3.5 w-3.5" />
                Create Your First Project
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {projects.map((p) => (
            <Card key={p.id} className="hover:border-foreground/20 transition-colors">
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <FolderKanban className="h-4 w-4 text-muted-foreground" />
                    <CardTitle className="text-sm font-medium">{p.name}</CardTitle>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <CardDescription className="flex items-center gap-1.5">
                  <Badge variant="secondary" className="font-mono text-[10px] px-1.5 py-0">
                    {p.slug}
                  </Badge>
                </CardDescription>
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Calendar className="h-3 w-3" />
                  Created {formatDate(p.created_at)}
                </div>
                <div className="text-xs font-mono text-muted-foreground/60 truncate">
                  {p.id}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Create Project Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FolderKanban className="h-4 w-4" />
              Create Project
            </DialogTitle>
            <DialogDescription>
              Projects group findings together and allow you to scope API tokens.
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleCreate} className="space-y-4 mt-2">
            <div>
              <label className="text-sm font-medium block mb-1.5">Project Name</label>
              <Input
                placeholder="e.g., Backend API"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                id="project-name-input"
                autoFocus
              />
              <p className="text-xs text-muted-foreground mt-1.5">
                A URL-friendly slug will be generated automatically from the name.
              </p>
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}

            <div className="flex justify-end gap-2 pt-2">
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={creating || !newName.trim()}>
                {creating ? "Creating..." : "Create Project"}
              </Button>
            </div>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  )
}
