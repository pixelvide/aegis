package api

import (
	"net/http"

	"github.com/pixelvide/aegis/server/internal/models"
)

// ─── Scans (read-only, legacy data) ─────────────────────────────────────────

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	projectId := pathParam(r, "projectId")
	scans, err := tenantStore(r).ListScans(r.Context(), projectId)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if scans == nil {
		scans = []models.Scan{}
	}
	writeResult(w, r, http.StatusOK, scans)
}

func (s *Server) handleGetScan(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	scan, err := tenantStore(r).GetScan(r.Context(), id)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if scan == nil {
		writeApiError(w, r, errResourceNotFound.WithMessage("Scan not found"))
		return
	}
	writeResult(w, r, http.StatusOK, scan)
}
