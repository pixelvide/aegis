import { createContext, useContext, useState, useEffect, useCallback } from "react"
import { projectsApi, setCurrentProject } from "@/lib/api"
import type { Project } from "@/lib/types"

interface ProjectContextValue {
  projects: Project[]
  currentProject: Project | null
  loading: boolean
  switchProject: (project: Project | null) => void
  refresh: () => void
}

const ProjectContext = createContext<ProjectContextValue>({
  projects: [],
  currentProject: null,
  loading: true,
  switchProject: () => {},
  refresh: () => {},
})

// eslint-disable-next-line react-refresh/only-export-components
export function useProject() {
  return useContext(ProjectContext)
}

export function ProjectProvider({ children }: { children: React.ReactNode }) {
  const [projects, setProjects] = useState<Project[]>([])
  const [currentProject, setCurrentProjectState] = useState<Project | null>(null)
  const [loading, setLoading] = useState(true)

  const loadProjects = useCallback(() => {
    projectsApi.list()
      .then((data) => {
        setProjects(data || [])
      })
      .catch(() => setProjects([]))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    loadProjects()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // React to URL changes to automatically select the project based on the route
  useEffect(() => {
    const handleUrlChange = () => {
      const match = window.location.pathname.match(/^\/project\/([^/]+)/)
      const projectIdFromUrl = match ? match[1] : null

      if (projectIdFromUrl && projects.length > 0) {
        const found = projects.find(p => p.id === projectIdFromUrl || p.slug === projectIdFromUrl)
        if (found && found.id !== currentProject?.id) {
          setCurrentProjectState(found)
          setCurrentProject(found.id)
        }
      } else if (!projectIdFromUrl && currentProject) {
        setCurrentProjectState(null)
        setCurrentProject(null)
      }
    }

    // Run on initial load and when projects change
    handleUrlChange()

    // Create a MutationObserver to watch for history changes since React Router handles them internally
    const pushState = history.pushState
    history.pushState = function (...args) {
      pushState.apply(history, args)
      window.dispatchEvent(new Event('popstate'))
    }
    
    window.addEventListener('popstate', handleUrlChange)
    return () => window.removeEventListener('popstate', handleUrlChange)
  }, [projects, currentProject])

  const switchProject = useCallback((project: Project | null) => {
    if (project) {
      window.location.href = `/project/${project.id}/dashboard`
    } else {
      window.location.href = `/`
    }
  }, [])

  return (
    <ProjectContext.Provider value={{ projects, currentProject, loading, switchProject, refresh: loadProjects }}>
      {children}
    </ProjectContext.Provider>
  )
}
