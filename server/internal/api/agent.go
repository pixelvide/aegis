package api

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

// ─── Request/Response types ─────────────────────────────────────────────────

// AgentFindingRequest is the body for POST /api/v1/agent/findings.
type AgentFindingRequest struct {
	ScanID      string  `json:"scan_id"`
	ProjectID   string  `json:"project_id"`
	Fingerprint string  `json:"fingerprint"`
	Title       string  `json:"title"`
	Severity    string  `json:"severity"`
	CWE         string  `json:"cwe,omitempty"`
	OWASP       string  `json:"owasp,omitempty"`
	CVE         string  `json:"cve,omitempty"`
	CVSSScore   float64 `json:"cvss_score,omitempty"`
	CVSSVector  string  `json:"cvss_vector,omitempty"`
	File        string  `json:"file,omitempty"`
	Line        int     `json:"line,omitempty"`
	Description string  `json:"description"`
	Source      string  `json:"source,omitempty"`
	Exploits    []struct {
		Filename string `json:"filename"`
		Language string `json:"language"`
		Code     string `json:"code"`
	} `json:"exploits,omitempty"`
}

// AgentFindingResponse wraps a finding with dedup info.
type AgentFindingResponse struct {
	*models.Finding `json:",inline"`
	Deduplicated    bool `json:"deduplicated"`
}

// AgentUpdateFindingRequest is the body for PATCH /api/v1/agent/findings/{id}.
type AgentUpdateFindingRequest struct {
	Status string `json:"status"`
}

// AgentExploitRequest is the body for POST /api/v1/agent/findings/{id}/exploits.
type AgentExploitRequest struct {
	Filename string `json:"filename"`
	Language string `json:"language"`
	Code     string `json:"code"`
}

// ─── Validation patterns ────────────────────────────────────────────────────

var (
	cwePattern = regexp.MustCompile(`^CWE-\d+$`)
	cvePattern = regexp.MustCompile(`^CVE-\d{4}-\d+$`)
)

// ─── Handlers ───────────────────────────────────────────────────────────────

func (s *Server) handleAgentCreateFinding(w http.ResponseWriter, r *http.Request) {
	var req AgentFindingRequest
	if err := decodeJSON(r, &req); err != nil {
		slog.Error("agent finding decode error", "error", err, "content_type", r.Header.Get("Content-Type"), "content_length", r.ContentLength)
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	// Validate required fields
	if req.ScanID == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("scan_id is required"))
		return
	}
	if len(req.ScanID) > 36 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("scan_id must be 36 chars or less"))
		return
	}
	if req.ProjectID == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("project_id is required"))
		return
	}
	if req.Fingerprint == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("fingerprint is required"))
		return
	}
	if len(req.Fingerprint) > 256 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("fingerprint must be 256 chars or less"))
		return
	}
	if req.Title == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("title is required"))
		return
	}
	if len(req.Title) > 500 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("title must be 500 chars or less"))
		return
	}
	if req.Severity == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("severity is required"))
		return
	}
	if req.Description == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("description is required"))
		return
	}
	if len(req.Description) > 100*1024 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("description must be 100KB or less"))
		return
	}

	// Validate severity
	sev := models.Severity(strings.ToLower(req.Severity))
	switch sev {
	case models.SeverityCritical, models.SeverityHigh, models.SeverityMedium, models.SeverityLow, models.SeverityInfo:
	default:
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("severity must be critical, high, medium, low, or info"))
		return
	}

	// Validate optional CWE format
	if req.CWE != "" && !cwePattern.MatchString(req.CWE) {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("invalid CWE format, expected CWE-NNN"))
		return
	}
	// Validate optional CVE format
	if req.CVE != "" && !cvePattern.MatchString(req.CVE) {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("invalid CVE format, expected CVE-YYYY-NNNNN"))
		return
	}
	// Validate CVSS score range
	if req.CVSSScore < 0 || req.CVSSScore > 10.0 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("cvss_score must be between 0.0 and 10.0"))
		return
	}

	// Sanitize file path (prevent traversal)
	if strings.Contains(req.File, "..") {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("file path must not contain '..'"))
		return
	}

	// Enforce token project scope
	token := middleware.AgentTokenFromContext(r.Context())
	if token != nil && token.ProjectID != "" && token.ProjectID != req.ProjectID {
		writeApiError(w, r, errPermissionDenied.WithMessage("token is scoped to a different project"))
		return
	}

	// Verify project exists
	ts := tenantStore(r)
	// TODO(enhancement): Add GetProjectByID to store interface for direct UUID lookup

	now := time.Now().UTC()
	finding := &models.Finding{
		ID:          uuid.New().String(),
		ScanID:      req.ScanID,
		ProjectID:   req.ProjectID,
		Fingerprint: req.Fingerprint,
		Title:       req.Title,
		Severity:    sev,
		CWE:         req.CWE,
		OWASP:       req.OWASP,
		CVE:         req.CVE,
		CVSSScore:   req.CVSSScore,
		CVSSVector:  req.CVSSVector,
		File:        req.File,
		Line:        req.Line,
		Status:      models.FindingOpen,
		Description: req.Description,
		Source:      req.Source,
		SeenCount:   1,
		LastSeenAt:  now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Upsert with fingerprint deduplication
	deduplicated, err := ts.UpsertFinding(r.Context(), finding)
	if err != nil {
		slog.Error("upsert finding failed", "error", err, "fingerprint", finding.Fingerprint, "project_id", finding.ProjectID)
		writeApiError(w, r, errServerInternal)
		return
	}

	// Create exploits if this is a new finding and exploits are provided
	if !deduplicated && len(req.Exploits) > 0 {
		for _, ex := range req.Exploits {
			if ex.Filename == "" || ex.Code == "" {
				continue
			}
			if len(ex.Code) > 500*1024 {
				continue // skip oversized exploits
			}
			exploit := &models.Exploit{
				ID:        uuid.New().String(),
				FindingID: finding.ID,
				Filename:  ex.Filename,
				Language:  ex.Language,
				Code:      ex.Code,
			}
			if createErr := ts.CreateExploit(r.Context(), exploit); createErr != nil {
				// Log but don't fail the request
				continue
			}
		}
	}

	status := http.StatusCreated
	if deduplicated {
		status = http.StatusOK
	}

	writeResult(w, r, status, AgentFindingResponse{
		Finding:      finding,
		Deduplicated: deduplicated,
	})
}

func (s *Server) handleAgentListFindings(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("project_id query parameter is required"))
		return
	}

	// Enforce token project scope
	token := middleware.AgentTokenFromContext(r.Context())
	if token != nil && token.ProjectID != "" && token.ProjectID != projectID {
		writeApiError(w, r, errPermissionDenied.WithMessage("token is scoped to a different project"))
		return
	}

	ts := tenantStore(r)
	findings, err := ts.ListFindings(r.Context(), findingFilterFromQuery(r, projectID))
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if findings == nil {
		findings = []models.Finding{}
	}
	writeResult(w, r, http.StatusOK, findings)
}

func (s *Server) handleAgentUpdateFinding(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")

	var req AgentUpdateFindingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	// Agents can only set: open (reopen) or verified (confirm fix)
	status := models.FindingStatus(req.Status)
	if status != models.FindingOpen && status != models.FindingVerified {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("agents can only set status to 'open' or 'verified'"))
		return
	}

	ts := tenantStore(r)

	// Verify finding exists
	existing, err := ts.GetFinding(r.Context(), id)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if existing == nil {
		writeApiError(w, r, errResourceNotFound.WithMessage("finding not found"))
		return
	}

	// Enforce token project scope
	token := middleware.AgentTokenFromContext(r.Context())
	if token != nil && token.ProjectID != "" && existing.ProjectID != "" && token.ProjectID != existing.ProjectID {
		writeApiError(w, r, errPermissionDenied.WithMessage("token is scoped to a different project"))
		return
	}

	if err := ts.UpdateFindingStatus(r.Context(), id, status); err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	existing.Status = status
	writeResult(w, r, http.StatusOK, existing)
}

func (s *Server) handleAgentCreateExploit(w http.ResponseWriter, r *http.Request) {
	findingID := pathParam(r, "id")

	var req AgentExploitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeApiError(w, r, errValidationInvalidBody)
		return
	}

	if req.Filename == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("filename is required"))
		return
	}
	if len(req.Filename) > 255 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("filename must be 255 chars or less"))
		return
	}
	if req.Code == "" {
		writeApiError(w, r, errValidationFieldRequired.WithMessage("code is required"))
		return
	}
	if len(req.Code) > 500*1024 {
		writeApiError(w, r, errValidationFieldInvalid.WithMessage("code must be 500KB or less"))
		return
	}

	ts := tenantStore(r)

	// Verify finding exists
	existing, err := ts.GetFinding(r.Context(), findingID)
	if err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}
	if existing == nil {
		writeApiError(w, r, errResourceNotFound.WithMessage("finding not found"))
		return
	}

	// Enforce token project scope
	token := middleware.AgentTokenFromContext(r.Context())
	if token != nil && token.ProjectID != "" && existing.ProjectID != "" && token.ProjectID != existing.ProjectID {
		writeApiError(w, r, errPermissionDenied.WithMessage("token is scoped to a different project"))
		return
	}

	exploit := &models.Exploit{
		ID:        uuid.New().String(),
		FindingID: findingID,
		Filename:  req.Filename,
		Language:  req.Language,
		Code:      req.Code,
	}

	if err := ts.CreateExploit(r.Context(), exploit); err != nil {
		writeApiError(w, r, errServerInternal)
		return
	}

	writeResult(w, r, http.StatusCreated, exploit)
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func findingFilterFromQuery(r *http.Request, projectID string) findingFilter {
	return findingFilter{
		ProjectID: projectID,
		Severity:  r.URL.Query().Get("severity"),
		Status:    r.URL.Query().Get("status"),
		CWE:       r.URL.Query().Get("cwe"),
	}
}

// findingFilter is a local type to construct store.FindingFilter.
type findingFilter = store.FindingFilter

