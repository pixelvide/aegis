package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pixelvide/localharness/adk"
)

// ── Reporter ─────────────────────────────────────────────────────────────────

// ReporterConfig holds the configuration for the finding reporter.
type ReporterConfig struct {
	BaseURL   string // Aegis server URL (e.g., https://acme.aegis.io)
	APIKey    string // API token (e.g., aegis_xxx) — from AEGIS_API_KEY env var
	ProjectID string // Project UUID
	ScanID    string // UUID v7 generated at agent startup
	Workspace string // Workspace directory (for local findings.json path)
}

// Reporter pushes findings to the Aegis server and writes them locally.
//
// Findings arrive via the `report_finding` host tool that the LLM calls
// directly with structured data. The reporter:
//  1. Always writes to .aegis/findings.json (local audit trail)
//  2. If server is configured, POSTs to /api/v1/agent/findings with retry
type Reporter struct {
	config     ReporterConfig
	httpClient *http.Client

	mu       sync.Mutex
	findings []LocalFinding

	// Counters for summary
	totalPushed   int
	totalFailed   int
	totalLocal    int
	serverEnabled bool
}

// LocalFinding is a finding stored in the local findings.json file.
type LocalFinding struct {
	Fingerprint  string  `json:"fingerprint"`
	Title        string  `json:"title"`
	Severity     string  `json:"severity"`
	CWE          string  `json:"cwe,omitempty"`
	OWASP        string  `json:"owasp,omitempty"`
	CVE          string  `json:"cve,omitempty"`
	CVSSScore    float64 `json:"cvss_score,omitempty"`
	File         string  `json:"file,omitempty"`
	Line         int     `json:"line,omitempty"`
	Description  string  `json:"description"`
	Source       string  `json:"source,omitempty"`
	ScanID       string  `json:"scan_id"`
	ProjectID    string  `json:"project_id"`
	ServerSynced bool    `json:"server_synced"`
	CreatedAt    string  `json:"created_at"`
}

// LocalFindingsFile is the top-level structure of findings.json.
type LocalFindingsFile struct {
	ScanID    string         `json:"scan_id"`
	StartedAt string        `json:"started_at"`
	Findings  []LocalFinding `json:"findings"`
}

// ServerFindingRequest is the request body for POST /api/v1/agent/findings.
type ServerFindingRequest struct {
	ScanID      string  `json:"scan_id"`
	ProjectID   string  `json:"project_id"`
	Fingerprint string  `json:"fingerprint"`
	Title       string  `json:"title"`
	Severity    string  `json:"severity"`
	CWE         string  `json:"cwe,omitempty"`
	OWASP       string  `json:"owasp,omitempty"`
	CVE         string  `json:"cve,omitempty"`
	CVSSScore   float64 `json:"cvss_score,omitempty"`
	File        string  `json:"file,omitempty"`
	Line        int     `json:"line,omitempty"`
	Description string  `json:"description"`
	Source      string  `json:"source,omitempty"`
}

// ── Constructor ──────────────────────────────────────────────────────────────

// NewReporter creates a new Reporter instance.
// If BaseURL is empty, server push is disabled (offline mode).
func NewReporter(cfg ReporterConfig) *Reporter {
	r := &Reporter{
		config:        cfg,
		serverEnabled: cfg.BaseURL != "" && cfg.APIKey != "",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if r.serverEnabled && !strings.HasPrefix(cfg.BaseURL, "https://") {
		log.Printf("⚠️  Warning: AEGIS_BASE_URL is not HTTPS (%s). Tokens will be sent in plaintext.", cfg.BaseURL)
	}

	return r
}

// IsServerEnabled returns true if the reporter is configured to push to a server.
func (r *Reporter) IsServerEnabled() bool {
	return r.serverEnabled
}

// ── Host Tool ────────────────────────────────────────────────────────────────

// HostTool returns a HostToolDef that registers `report_finding` as a custom
// tool the LLM can call directly with structured finding data.
//
// This is the primary integration point: personas call report_finding() when
// they discover a vulnerability, and the handler pushes it to the server
// and/or writes it locally — no file scraping or hooks needed.
func (r *Reporter) HostTool() adk.HostToolDef {
	return adk.HostToolDef{
		Name:        "report_finding",
		Description: "Report a security finding to the Aegis platform. Call this tool for EVERY vulnerability you discover. The finding will be pushed to the Aegis server (if configured) and saved locally to findings.json.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"fingerprint": map[string]any{
					"type":        "string",
					"description": "Unique identifier for deduplication (e.g., 'sha256:abc123' or 'AEGIS-001'). Must be deterministic for the same finding.",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Short, descriptive title (e.g., 'SQL Injection in UserController.login')",
				},
				"severity": map[string]any{
					"type":        "string",
					"enum":        []string{"critical", "high", "medium", "low", "info"},
					"description": "Severity level of the finding",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Detailed explanation of the vulnerability, impact, and remediation steps",
				},
				"file": map[string]any{
					"type":        "string",
					"description": "File path where the vulnerability was found (relative to workspace root)",
				},
				"line": map[string]any{
					"type":        "integer",
					"description": "Line number where the vulnerability was found",
				},
				"cwe": map[string]any{
					"type":        "string",
					"description": "CWE identifier (e.g., 'CWE-89' for SQL Injection)",
				},
				"owasp": map[string]any{
					"type":        "string",
					"description": "OWASP category (e.g., 'A03:2021 - Injection')",
				},
				"cve": map[string]any{
					"type":        "string",
					"description": "CVE identifier if known (e.g., 'CVE-2024-1234')",
				},
				"cvss_score": map[string]any{
					"type":        "number",
					"description": "CVSS v3.1 score (0.0 - 10.0)",
				},
			},
			"required": []string{"fingerprint", "title", "severity", "description"},
		},
		Handler: r.handleReportFinding,
	}
}

// handleReportFinding is the HostToolDef handler called by the LLM.
func (r *Reporter) handleReportFinding(_ context.Context, args map[string]any) (any, error) {
	// Extract required fields
	fingerprint, _ := args["fingerprint"].(string)
	title, _ := args["title"].(string)
	severity, _ := args["severity"].(string)
	description, _ := args["description"].(string)

	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if fingerprint == "" {
		return nil, fmt.Errorf("fingerprint is required")
	}

	// Extract optional fields
	file, _ := args["file"].(string)
	cwe, _ := args["cwe"].(string)
	owasp, _ := args["owasp"].(string)
	cve, _ := args["cve"].(string)

	var line int
	if v, ok := args["line"].(float64); ok {
		line = int(v)
	}

	var cvssScore float64
	if v, ok := args["cvss_score"].(float64); ok {
		cvssScore = v
	}

	finding := LocalFinding{
		Fingerprint: fingerprint,
		Title:       title,
		Severity:    strings.ToLower(severity),
		Description: description,
		File:        file,
		Line:        line,
		CWE:         cwe,
		OWASP:       owasp,
		CVE:         cve,
		CVSSScore:   cvssScore,
		Source:      "aegis-agent",
		ScanID:      r.config.ScanID,
		ProjectID:   r.config.ProjectID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	// Push to server (if configured)
	if r.serverEnabled {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		if err := r.pushToServer(ctx, finding); err != nil {
			log.Printf("⚠️  Failed to push finding to server: %v (saved locally)", err)
			finding.ServerSynced = false
			r.mu.Lock()
			r.totalFailed++
			r.mu.Unlock()
		} else {
			finding.ServerSynced = true
			r.mu.Lock()
			r.totalPushed++
			r.mu.Unlock()
		}
	}

	// Always save locally
	r.mu.Lock()
	r.findings = append(r.findings, finding)
	r.totalLocal++
	r.mu.Unlock()

	// Return confirmation to the LLM
	result := map[string]any{
		"status":        "recorded",
		"fingerprint":   finding.Fingerprint,
		"scan_id":       finding.ScanID,
		"server_synced": finding.ServerSynced,
	}
	if r.serverEnabled {
		if finding.ServerSynced {
			result["message"] = "Finding pushed to server and saved locally."
		} else {
			result["message"] = "Finding saved locally (server push failed — will retry on next sync)."
		}
	} else {
		result["message"] = "Finding saved locally (server not configured)."
	}
	return result, nil
}

// ── Server Push ──────────────────────────────────────────────────────────────

// pushToServer sends a finding to the Aegis server with retry.
// Retry: 3 attempts with exponential backoff (1s, 2s, 4s).
func (r *Reporter) pushToServer(ctx context.Context, finding LocalFinding) error {
	req := ServerFindingRequest{
		ScanID:      finding.ScanID,
		ProjectID:   finding.ProjectID,
		Fingerprint: finding.Fingerprint,
		Title:       finding.Title,
		Severity:    finding.Severity,
		CWE:         finding.CWE,
		OWASP:       finding.OWASP,
		CVE:         finding.CVE,
		CVSSScore:   finding.CVSSScore,
		File:        finding.File,
		Line:        finding.Line,
		Description: finding.Description,
		Source:      finding.Source,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal finding: %w", err)
	}

	url := strings.TrimRight(r.config.BaseURL, "/") + "/api/v1/agent/findings"

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+r.config.APIKey)

		resp, err := r.httpClient.Do(httpReq)
		if err != nil {
			log.Printf("  Attempt %d/%d failed: %v", attempt+1, maxRetries, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil // Success
		}

		// Don't retry on client errors (4xx) except 429 (rate limit)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
			return fmt.Errorf("server returned %d (not retrying)", resp.StatusCode)
		}

		log.Printf("  Attempt %d/%d: server returned %d", attempt+1, maxRetries, resp.StatusCode)
	}

	return fmt.Errorf("all %d attempts failed", maxRetries)
}

// ── Local File ───────────────────────────────────────────────────────────────

// Close flushes all accumulated findings to the local findings.json file.
// Must be called before the agent exits.
func (r *Reporter) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.findings) == 0 {
		return nil
	}

	// Determine output path
	wsDir := r.config.Workspace
	if wsDir == "" {
		wsDir, _ = os.Getwd()
	}
	outDir := filepath.Join(wsDir, ".aegis")
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("create .aegis directory: %w", err)
	}
	outPath := filepath.Join(outDir, "findings.json")

	file := LocalFindingsFile{
		ScanID:    r.config.ScanID,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Findings:  r.findings,
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal findings: %w", err)
	}

	if err := os.WriteFile(outPath, data, 0o640); err != nil {
		return fmt.Errorf("write findings.json: %w", err)
	}

	log.Printf("📄 Findings written to %s (%d findings)", outPath, len(r.findings))
	return nil
}

// SummaryLine returns a one-line summary of reporting activity.
func (r *Reporter) SummaryLine() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.serverEnabled {
		return fmt.Sprintf("%d local", r.totalLocal)
	}
	return fmt.Sprintf("%d pushed, %d failed, %d local", r.totalPushed, r.totalFailed, r.totalLocal)
}
