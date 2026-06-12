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
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	if req.Name == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("Name is required"))
		return
	}

	slug := store.SanitizeSlug(req.Slug)
	if slug == "" {
		slug = store.SanitizeSlug(req.Name)
	}
	if len(slug) < 2 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("Slug must be at least 2 characters"))
		return
	}

	project := &models.Project{
		Name: req.Name,
		Slug: slug,
	}

	ts := tenantStore(r)
	if err := ts.CreateProject(r.Context(), project); err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	writeResult(w, r, http.StatusCreated, project)
}

// handleListProjects returns all projects in the current org.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := tenantStore(r).ListProjects(r.Context())
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if projects == nil {
		projects = []models.Project{}
	}
	writeResult(w, r, http.StatusOK, projects)
}

// handleGetProject returns a project by slug.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	slug := pathParam(r, "slug")
	project, err := tenantStore(r).GetProjectBySlug(r.Context(), slug)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if project == nil {
		writeApiError(w, r, errResourceNotFound.WithMessage("Project not found"))
		return
	}
	writeResult(w, r, http.StatusOK, project)
}
