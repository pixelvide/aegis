package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/pixelvide/aegis/server/internal/middleware"
	"github.com/pixelvide/aegis/server/internal/models"
	"github.com/pixelvide/aegis/server/internal/store"
)

// ─── Request/Response types ─────────────────────────────────────────────────

// AgentFindingRequest is the body for POST /api/v1/agent/findings.
type AgentFindingRequest struct {
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
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate required fields
	if req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	if req.Fingerprint == "" {
		writeError(w, http.StatusBadRequest, "fingerprint is required")
		return
	}
	if len(req.Fingerprint) > 256 {
		writeError(w, http.StatusBadRequest, "fingerprint must be 256 chars or less")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if len(req.Title) > 500 {
		writeError(w, http.StatusBadRequest, "title must be 500 chars or less")
		return
	}
	if req.Severity == "" {
		writeError(w, http.StatusBadRequest, "severity is required")
		return
	}
	if req.Description == "" {
		writeError(w, http.StatusBadRequest, "description is required")
		return
	}
	if len(req.Description) > 100*1024 {
		writeError(w, http.StatusBadRequest, "description must be 100KB or less")
		return
	}

	// Validate severity
	sev := models.Severity(strings.ToLower(req.Severity))
	switch sev {
	case models.SeverityCritical, models.SeverityHigh, models.SeverityMedium, models.SeverityLow, models.SeverityInfo:
	default:
		writeError(w, http.StatusBadRequest, "severity must be critical, high, medium, low, or info")
		return
	}

	// Validate optional CWE format
	if req.CWE != "" && !cwePattern.MatchString(req.CWE) {
		writeError(w, http.StatusBadRequest, "invalid CWE format, expected CWE-NNN")
		return
	}
	// Validate optional CVE format
	if req.CVE != "" && !cvePattern.MatchString(req.CVE) {
		writeError(w, http.StatusBadRequest, "invalid CVE format, expected CVE-YYYY-NNNNN")
		return
	}
	// Validate CVSS score range
	if req.CVSSScore < 0 || req.CVSSScore > 10.0 {
		writeError(w, http.StatusBadRequest, "cvss_score must be between 0.0 and 10.0")
		return
	}

	// Sanitize file path (prevent traversal)
	if strings.Contains(req.File, "..") {
		writeError(w, http.StatusBadRequest, "file path must not contain '..'")
		return
	}

	// Enforce token project scope
	token := middleware.AgentTokenFromContext(r.Context())
	if token != nil && token.ProjectID != "" && token.ProjectID != req.ProjectID {
		writeError(w, http.StatusForbidden, "token is scoped to a different project")
		return
	}

	// Verify project exists
	ts := tenantStore(r)
	// TODO(enhancement): Add GetProjectByID to store interface for direct UUID lookup

	now := time.Now().UTC()
	finding := &models.Finding{
		ID:          uuid.New().String(),
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
		writeError(w, http.StatusInternalServerError, "failed to save finding")
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

	writeJSON(w, status, AgentFindingResponse{
		Finding:      finding,
		Deduplicated: deduplicated,
	})
}

func (s *Server) handleAgentListFindings(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project_id query parameter is required")
		return
	}

	// Enforce token project scope
	token := middleware.AgentTokenFromContext(r.Context())
	if token != nil && token.ProjectID != "" && token.ProjectID != projectID {
		writeError(w, http.StatusForbidden, "token is scoped to a different project")
		return
	}

	ts := tenantStore(r)
	findings, err := ts.ListFindings(r.Context(), findingFilterFromQuery(r, projectID))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list findings")
		return
	}
	if findings == nil {
		findings = []models.Finding{}
	}
	writeJSON(w, http.StatusOK, findings)
}

func (s *Server) handleAgentUpdateFinding(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")

	var req AgentUpdateFindingRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Agents can only set: open (reopen) or verified (confirm fix)
	status := models.FindingStatus(req.Status)
	if status != models.FindingOpen && status != models.FindingVerified {
		writeError(w, http.StatusBadRequest, "agents can only set status to 'open' or 'verified'")
		return
	}

	ts := tenantStore(r)

	// Verify finding exists
	existing, err := ts.GetFinding(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get finding")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "finding not found")
		return
	}

	// Enforce token project scope
	token := middleware.AgentTokenFromContext(r.Context())
	if token != nil && token.ProjectID != "" && existing.ProjectID != "" && token.ProjectID != existing.ProjectID {
		writeError(w, http.StatusForbidden, "token is scoped to a different project")
		return
	}

	if err := ts.UpdateFindingStatus(r.Context(), id, status); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update finding")
		return
	}

	existing.Status = status
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) handleAgentCreateExploit(w http.ResponseWriter, r *http.Request) {
	findingID := pathParam(r, "id")

	var req AgentExploitRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Filename == "" {
		writeError(w, http.StatusBadRequest, "filename is required")
		return
	}
	if len(req.Filename) > 255 {
		writeError(w, http.StatusBadRequest, "filename must be 255 chars or less")
		return
	}
	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}
	if len(req.Code) > 500*1024 {
		writeError(w, http.StatusBadRequest, "code must be 500KB or less")
		return
	}

	ts := tenantStore(r)

	// Verify finding exists
	existing, err := ts.GetFinding(r.Context(), findingID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get finding")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "finding not found")
		return
	}

	// Enforce token project scope
	token := middleware.AgentTokenFromContext(r.Context())
	if token != nil && token.ProjectID != "" && existing.ProjectID != "" && token.ProjectID != existing.ProjectID {
		writeError(w, http.StatusForbidden, "token is scoped to a different project")
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
		writeError(w, http.StatusInternalServerError, "failed to create exploit")
		return
	}

	writeJSON(w, http.StatusCreated, exploit)
}

// ─── Token Management (user-facing) ─────────────────────────────────────────

// CreateTokenRequest is the body for POST /api/v1/tokens.
type CreateTokenRequest struct {
	Name      string `json:"name"`
	ProjectID string `json:"project_id,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"` // days, 0 = never
}

// CreateTokenResponse returns the plaintext token once and token metadata.
type CreateTokenResponse struct {
	Token string          `json:"token"` // plaintext — shown ONCE, never stored
	Info  models.APIToken `json:"info"`
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req CreateTokenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Name) > 100 {
		writeError(w, http.StatusBadRequest, "name must be 100 chars or less")
		return
	}

	// Generate 32 random hex bytes → "aegis_<hex>"
	rawBytes := make([]byte, 16)
	if _, err := rand.Read(rawBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	plaintext := "aegis_" + hex.EncodeToString(rawBytes)

	// bcrypt hash (cost 12)
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), 12)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash token")
		return
	}

	// Extract prefix for lookup (first 14 chars: "aegis_" + 8 hex)
	prefix := plaintext[:14]

	user := middleware.UserFromContext(r.Context())

	now := time.Now().UTC()
	token := &models.APIToken{
		ID:        uuid.New().String(),
		ProjectID: req.ProjectID,
		Name:      req.Name,
		Prefix:    prefix,
		CreatedBy: user.ID,
		CreatedAt: now,
	}

	if req.ExpiresIn > 0 {
		exp := now.Add(time.Duration(req.ExpiresIn) * 24 * time.Hour)
		token.ExpiresAt = &exp
	}

	ts := tenantStore(r)
	if err := ts.CreateAPIToken(r.Context(), token, string(hash)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	writeJSON(w, http.StatusCreated, CreateTokenResponse{
		Token: plaintext,
		Info:  *token,
	})
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	ts := tenantStore(r)
	tokens, err := ts.ListAPITokens(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tokens")
		return
	}
	if tokens == nil {
		tokens = []models.APIToken{}
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	ts := tenantStore(r)
	if err := ts.RevokeAPIToken(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
