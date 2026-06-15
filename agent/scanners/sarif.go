package scanners

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pixelvide/aegis/agent/pipeline"
)

// ── SARIF Parser ─────────────────────────────────────────────────────────────
//
// SARIF (Static Analysis Results Interchange Format) is an OASIS standard
// for expressing the output of static analysis tools. Most modern scanners
// support it:
//
//   semgrep  → --sarif
//   CodeQL   → --format=sarif-latest
//   ESLint   → @microsoft/eslint-formatter-sarif
//   Bandit   → -f sarif
//   Checkov  → --output sarif
//   Trivy    → --format sarif
//   Gosec    → -fmt=sarif

// SARIFLog is the top-level SARIF v2.1.0 structure.
// We only parse the fields we need for finding extraction.
type SARIFLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []SARIFRun `json:"runs"`
}

// SARIFRun represents a single run of a tool.
type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

// SARIFTool describes the tool that produced the run.
type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

// SARIFDriver is the main tool component.
type SARIFDriver struct {
	Name    string      `json:"name"`
	Version string      `json:"version"`
	Rules   []SARIFRule `json:"rules"`
}

// SARIFRule defines a static analysis rule.
type SARIFRule struct {
	ID               string                 `json:"id"`
	ShortDescription SARIFMessage           `json:"shortDescription"`
	FullDescription  SARIFMessage           `json:"fullDescription"`
	Help             SARIFMessage           `json:"help"`
	Properties       map[string]interface{} `json:"properties"`
}

// SARIFResult is a single finding from a run.
type SARIFResult struct {
	RuleID    string            `json:"ruleId"`
	RuleIndex int               `json:"ruleIndex"`
	Level     string            `json:"level"`
	Message   SARIFMessage      `json:"message"`
	Locations []SARIFLocation   `json:"locations"`
	Fixes     []SARIFFix        `json:"fixes"`
	Properties map[string]interface{} `json:"properties"`
}

// SARIFMessage holds a human-readable text or markdown message.
type SARIFMessage struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown"`
}

// SARIFLocation points to a physical location in a file.
type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"`
}

// SARIFPhysicalLocation describes a file path and region.
type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"`
	Region           SARIFRegion           `json:"region"`
}

// SARIFArtifactLocation is a reference to a file.
type SARIFArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId"`
}

// SARIFRegion describes a region within a file.
type SARIFRegion struct {
	StartLine   int            `json:"startLine"`
	EndLine     int            `json:"endLine"`
	StartColumn int            `json:"startColumn"`
	EndColumn   int            `json:"endColumn"`
	Snippet     SARIFSnippet   `json:"snippet"`
}

// SARIFSnippet contains the code at the flagged location.
type SARIFSnippet struct {
	Text string `json:"text"`
}

// SARIFFix describes a suggested fix for a result.
type SARIFFix struct {
	Description SARIFMessage `json:"description"`
}

// ── Parser ───────────────────────────────────────────────────────────────────

// ParseSARIF parses a SARIF JSON document and converts all results into
// normalized RawFindings. The workspace prefix is stripped from file paths.
func ParseSARIF(data []byte, workspacePrefix string) ([]pipeline.RawFinding, error) {
	var log SARIFLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("invalid SARIF JSON: %w", err)
	}

	var findings []pipeline.RawFinding

	for _, run := range log.Runs {
		// Build rule lookup for enrichment.
		rules := make(map[string]SARIFRule)
		for _, r := range run.Tool.Driver.Rules {
			rules[r.ID] = r
		}

		toolName := run.Tool.Driver.Name

		for _, result := range run.Results {
			f := convertResult(result, rules, toolName, workspacePrefix)
			if f.Title != "" {
				findings = append(findings, f)
			}
		}
	}

	return findings, nil
}

// convertResult transforms a single SARIF result into a RawFinding.
func convertResult(result SARIFResult, rules map[string]SARIFRule, toolName, workspacePrefix string) pipeline.RawFinding {
	f := pipeline.RawFinding{
		Title:       result.Message.Text,
		Description: result.Message.Text,
		Severity:    mapSARIFLevel(result.Level),
		Confidence:  "medium", // SARIF doesn't have confidence; default to medium.
		Source:      toolName,
	}

	// Extract location info.
	if len(result.Locations) > 0 {
		loc := result.Locations[0].PhysicalLocation
		f.FilePath = normalizeURI(loc.ArtifactLocation.URI, workspacePrefix)
		f.StartLine = loc.Region.StartLine
		f.EndLine = loc.Region.EndLine
		if f.EndLine == 0 {
			f.EndLine = f.StartLine
		}
		f.VulnerableCode = loc.Region.Snippet.Text
	}

	// Enrich from rule metadata.
	if rule, ok := rules[result.RuleID]; ok {
		// Prefer rule's short description as title (more descriptive).
		if rule.ShortDescription.Text != "" {
			f.Title = rule.ShortDescription.Text
		}

		// Use full description for finding description.
		if rule.FullDescription.Text != "" {
			f.Description = rule.FullDescription.Text
		}

		// Extract CWE from rule properties or tags.
		f.CWE = extractCWE(result.RuleID, rule.Properties)

		// Use fix description as remediation.
		if rule.Help.Text != "" {
			f.Remediation = rule.Help.Text
		}
	}

	// Fallback CWE extraction from the result itself.
	if f.CWE == "" {
		f.CWE = extractCWE(result.RuleID, result.Properties)
	}

	// Use fix description from result if available.
	if f.Remediation == "" && len(result.Fixes) > 0 {
		f.Remediation = result.Fixes[0].Description.Text
	}

	return f
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// mapSARIFLevel converts a SARIF level to our severity scale.
func mapSARIFLevel(level string) string {
	switch strings.ToLower(level) {
	case "error":
		return "high"
	case "warning":
		return "medium"
	case "note":
		return "low"
	case "none":
		return "info"
	default:
		return "medium"
	}
}

// normalizeURI strips file:// prefix and workspace prefix from a SARIF URI.
func normalizeURI(uri, workspacePrefix string) string {
	// Strip file:// scheme.
	uri = strings.TrimPrefix(uri, "file://")
	uri = strings.TrimPrefix(uri, "file:")

	// Strip workspace prefix.
	if workspacePrefix != "" {
		wp := strings.TrimSuffix(workspacePrefix, "/") + "/"
		uri = strings.TrimPrefix(uri, wp)
	}

	// Strip leading slash for relative paths.
	uri = strings.TrimPrefix(uri, "/")

	return uri
}

// extractCWE attempts to find a CWE identifier from rule properties.
// Semgrep stores CWEs in properties.tags, CodeQL in properties.cwe.
func extractCWE(ruleID string, properties map[string]interface{}) string {
	if properties == nil {
		return ""
	}

	// Check properties.cwe (CodeQL style).
	if cwe, ok := properties["cwe"].(string); ok {
		return normalizeCWEStr(cwe)
	}

	// Check properties.cwe as array (CodeQL sometimes uses this).
	if cweList, ok := properties["cwe"].([]interface{}); ok && len(cweList) > 0 {
		if cwe, ok := cweList[0].(string); ok {
			return normalizeCWEStr(cwe)
		}
	}

	// Check properties.tags (semgrep style: ["cwe:CWE-89", "owasp:A03"]).
	if tags, ok := properties["tags"].([]interface{}); ok {
		for _, tag := range tags {
			s, ok := tag.(string)
			if !ok {
				continue
			}
			// Match "CWE-NNN" or "cwe:CWE-NNN" or "cwe:89".
			s = strings.TrimPrefix(s, "cwe:")
			if strings.HasPrefix(strings.ToUpper(s), "CWE-") {
				return strings.ToUpper(s)
			}
			// Bare number after "cwe:".
			if len(s) > 0 && s[0] >= '0' && s[0] <= '9' {
				return "CWE-" + s
			}
		}
	}

	// Check properties.security-severity (GitHub CodeQL).
	// This doesn't give CWE but is another signal.

	return ""
}

// normalizeCWEStr normalizes various CWE string formats to "CWE-NNN".
func normalizeCWEStr(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	// Already correct format.
	if strings.HasPrefix(s, "CWE-") {
		return s
	}

	// "CWE 89" or "CWE89" → "CWE-89".
	s = strings.TrimPrefix(s, "CWE")
	s = strings.TrimSpace(s)
	if s != "" {
		return "CWE-" + s
	}

	return ""
}
