package scanners

import (
	"context"
	"fmt"
	"strings"

	"github.com/pixelvide/aegis/agent/pipeline"
	"github.com/pixelvide/localharness/adk"
)

// ── Host Tool ────────────────────────────────────────────────────────────────
//
// Exposes the scanner registry as a host tool that the LLM agent can invoke.
// This lets the agent run external scanners as part of its investigation:
//
//   Agent: "Let me run semgrep to check for injection patterns"
//   → calls run_scanner(scanner="semgrep")
//   → returns normalized findings as structured JSON
//
// The agent can then use these findings to prioritize its own analysis,
// cross-reference with its LLM-based findings, or report them directly.

// HostTool returns a HostToolDef that registers `run_scanner` as a custom
// tool the LLM agent can call to invoke external static analysis tools.
func (r *Registry) HostTool(workspace string) adk.HostToolDef {
	return adk.HostToolDef{
		Name: "run_scanner",
		Description: "Run an external static analysis scanner (semgrep, opengrep, trivy, bandit, gosec) " +
			"and return normalized security findings. Use 'list' as the scanner name to see " +
			"which scanners are available on this system. Results are returned as structured " +
			"findings with file paths, line numbers, CWE IDs, and severity levels.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scanner": map[string]any{
					"type":        "string",
					"description": "Scanner to run: 'semgrep', 'trivy', 'bandit', 'gosec', or 'all' to run every available scanner. Use 'list' to see what's installed.",
				},
			},
			"required": []string{"scanner"},
		},
		Handler: func(ctx context.Context, args map[string]any) (any, error) {
			scannerName, _ := args["scanner"].(string)
			scannerName = strings.ToLower(strings.TrimSpace(scannerName))

			if scannerName == "" {
				return nil, fmt.Errorf("scanner name is required")
			}

			// Handle "list" command.
			if scannerName == "list" {
				return r.listScannersResult(), nil
			}

			// Handle "all" — run every available scanner.
			if scannerName == "all" {
				findings, err := r.RunAll(ctx, workspace)
				if err != nil {
					return nil, err
				}
				return formatHostToolResult(findings), nil
			}

			// Run specific scanner.
			findings, err := r.Run(ctx, scannerName, workspace)
			if err != nil {
				return nil, err
			}

			return formatHostToolResult(findings), nil
		},
	}
}

// listScannersResult returns the scanner availability info as structured data.
func (r *Registry) listScannersResult() map[string]any {
	type scannerStatus struct {
		Name      string `json:"name"`
		Available bool   `json:"available"`
	}

	var scanners []scannerStatus
	for _, s := range r.All() {
		scanners = append(scanners, scannerStatus{
			Name:      s.Name(),
			Available: s.Available(),
		})
	}

	return map[string]any{
		"scanners": scanners,
		"summary":  r.ListInfo(),
	}
}

// formatHostToolResult converts pipeline findings into a structured response
// that the LLM can easily parse and act on.
func formatHostToolResult(findings []pipeline.RawFinding) map[string]any {
	type findingOut struct {
		Title          string `json:"title"`
		Severity       string `json:"severity"`
		CWE            string `json:"cwe,omitempty"`
		File           string `json:"file"`
		StartLine      int    `json:"start_line"`
		EndLine        int    `json:"end_line,omitempty"`
		Description    string `json:"description"`
		VulnerableCode string `json:"vulnerable_code,omitempty"`
		Remediation    string `json:"remediation,omitempty"`
		Source         string `json:"source"`
	}

	out := make([]findingOut, 0, len(findings))
	for _, f := range findings {
		out = append(out, findingOut{
			Title:          f.Title,
			Severity:       f.Severity,
			CWE:            f.CWE,
			File:           f.FilePath,
			StartLine:      f.StartLine,
			EndLine:        f.EndLine,
			Description:    f.Description,
			VulnerableCode: f.VulnerableCode,
			Remediation:    f.Remediation,
			Source:         f.Source,
		})
	}

	return map[string]any{
		"findings_count": len(out),
		"findings":       out,
	}
}
