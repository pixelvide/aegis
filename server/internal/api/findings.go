package api

import (
	"net/http"

	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

func (s *Server) handleListFindings(w http.ResponseWriter, r *http.Request) {
	filter := store.FindingFilter{
		ScanID:   queryParam(r, "scan_id"),
		Severity: queryParam(r, "severity"),
		Status:   queryParam(r, "status"),
		CWE:      queryParam(r, "cwe"),
	}

	findings, err := tenantStore(r).ListFindings(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list findings")
		return
	}

	// Load exploits for each finding
	for i := range findings {
		exploits, err := tenantStore(r).ListExploits(r.Context(), findings[i].ID)
		if err == nil {
			findings[i].Exploits = exploits
		}
	}

	if findings == nil {
		findings = []models.Finding{}
	}
	writeJSON(w, http.StatusOK, findings)
}

func (s *Server) handleGetFinding(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	finding, err := tenantStore(r).GetFinding(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get finding")
		return
	}
	if finding == nil {
		writeError(w, http.StatusNotFound, "finding not found")
		return
	}

	// Load exploits
	exploits, err := tenantStore(r).ListExploits(r.Context(), finding.ID)
	if err == nil {
		finding.Exploits = exploits
	}

	writeJSON(w, http.StatusOK, finding)
}

func (s *Server) handleUpdateFinding(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")

	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate status against allowlist
	status := models.FindingStatus(req.Status)
	switch status {
	case models.FindingOpen, models.FindingConfirmed, models.FindingFixed,
		models.FindingFalsePositive, models.FindingWontFix:
		// Valid
	default:
		writeError(w, http.StatusBadRequest, "invalid status: must be open, confirmed, fixed, false_positive, or wontfix")
		return
	}

	if err := tenantStore(r).UpdateFindingStatus(r.Context(), id, status); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update finding")
		return
	}

	// Return updated finding
	finding, _ := tenantStore(r).GetFinding(r.Context(), id)
	if finding == nil {
		writeError(w, http.StatusNotFound, "finding not found")
		return
	}
	writeJSON(w, http.StatusOK, finding)
}

func (s *Server) handleListExploits(w http.ResponseWriter, r *http.Request) {
	findingID := pathParam(r, "id")
	exploits, err := tenantStore(r).ListExploits(r.Context(), findingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list exploits")
		return
	}
	if exploits == nil {
		exploits = []models.Exploit{}
	}
	writeJSON(w, http.StatusOK, exploits)
}

func (s *Server) handleGetExploit(w http.ResponseWriter, r *http.Request) {
	eid := pathParam(r, "eid")
	exploit, err := tenantStore(r).GetExploit(r.Context(), eid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get exploit")
		return
	}
	if exploit == nil {
		writeError(w, http.StatusNotFound, "exploit not found")
		return
	}
	writeJSON(w, http.StatusOK, exploit)
}
