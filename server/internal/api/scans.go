package api

import (
	"net/http"

	"github.com/pixelvide/aegis/server/internal/models"
)

// ─── Scans (read-only, legacy data) ─────────────────────────────────────────

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	scans, err := tenantStore(r).ListScans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list scans")
		return
	}
	if scans == nil {
		scans = []models.Scan{}
	}
	writeJSON(w, http.StatusOK, scans)
}

func (s *Server) handleGetScan(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	scan, err := tenantStore(r).GetScan(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get scan")
		return
	}
	if scan == nil {
		writeError(w, http.StatusNotFound, "scan not found")
		return
	}
	writeJSON(w, http.StatusOK, scan)
}
