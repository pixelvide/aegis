package api

import (
	"net/http"

	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

type createProjectRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// handleCreateProject creates a new project in the current org.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	slug := store.SanitizeSlug(req.Slug)
	if slug == "" {
		slug = store.SanitizeSlug(req.Name)
	}
	if len(slug) < 2 {
		writeError(w, http.StatusBadRequest, "slug must be at least 2 characters")
		return
	}

	project := &models.Project{
		Name: req.Name,
		Slug: slug,
	}

	ts := tenantStore(r)
	if err := ts.CreateProject(r.Context(), project); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create project: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

// handleListProjects returns all projects in the current org.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := tenantStore(r).ListProjects(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}
	if projects == nil {
		projects = []models.Project{}
	}
	writeJSON(w, http.StatusOK, projects)
}

// handleGetProject returns a project by slug.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	slug := pathParam(r, "slug")
	project, err := tenantStore(r).GetProjectBySlug(r.Context(), slug)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get project")
		return
	}
	if project == nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, project)
}
