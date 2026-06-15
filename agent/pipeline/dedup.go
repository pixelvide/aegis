package pipeline

import (
	"fmt"
	"sort"
	"strings"
)

// ── Deduplicator ─────────────────────────────────────────────────────────────

// Deduplicator performs deterministic deduplication on raw scanner findings.
//
// Dedup rules (applied in order):
//  1. Exact match: same file + line + CWE → merge, keep highest confidence
//  2. Near match: same file + CWE within 5 lines → merge (likely same root cause)
//  3. Cross-chunk overlap: same vulnerability from overlapping chunk ranges → merge
//  4. Best-of-K merge: from K attempts on the same chunk, union unique findings
type Deduplicator struct {
	// NearMatchThreshold is the maximum line distance for near-match dedup.
	// Findings with the same file+CWE within this many lines are merged.
	NearMatchThreshold int
}

// NewDeduplicator creates a Deduplicator with default settings.
func NewDeduplicator() *Deduplicator {
	return &Deduplicator{
		NearMatchThreshold: 5,
	}
}

// Dedup takes a slice of raw findings from all scanner subagents and returns
// a deduplicated slice. The order is stable: findings are sorted by file path
// then by start line.
func (d *Deduplicator) Dedup(raw []RawFinding) []DedupedFinding {
	if len(raw) == 0 {
		return nil
	}

	// Group findings by dedup key (file + normalized CWE + line bucket).
	groups := d.groupFindings(raw)

	// Convert each group into a DedupedFinding.
	var results []DedupedFinding
	for _, group := range groups {
		results = append(results, d.mergeGroup(group))
	}

	// Sort by file path, then by start line for stable output.
	sort.Slice(results, func(i, j int) bool {
		if results[i].FilePath != results[j].FilePath {
			return results[i].FilePath < results[j].FilePath
		}
		return results[i].StartLine < results[j].StartLine
	})

	// Assign stable IDs after sorting.
	for i := range results {
		results[i].ID = fmt.Sprintf("DEDUP-%03d", i+1)
	}

	return results
}

// dedupKey is the grouping key for near-match deduplication.
type dedupKey struct {
	filePath  string
	cwe       string
	lineBucket int // start_line / threshold (quantized)
}

// groupFindings groups raw findings by their dedup key.
// Findings with the same file+CWE within NearMatchThreshold lines
// are placed in the same group.
func (d *Deduplicator) groupFindings(raw []RawFinding) map[dedupKey][]RawFinding {
	groups := make(map[dedupKey][]RawFinding)

	for _, f := range raw {
		threshold := d.NearMatchThreshold
		if threshold <= 0 {
			threshold = 1
		}

		key := dedupKey{
			filePath:  f.FilePath,
			cwe:       normalizeCWE(f.CWE),
			lineBucket: f.StartLine / threshold,
		}
		groups[key] = append(groups[key], f)
	}

	return groups
}

// mergeGroup merges a group of raw findings into a single DedupedFinding.
// It picks the best title, description, severity, and confidence from the group.
func (d *Deduplicator) mergeGroup(group []RawFinding) DedupedFinding {
	if len(group) == 0 {
		return DedupedFinding{}
	}

	// Pick the finding with the highest severity+confidence as the primary.
	sort.Slice(group, func(i, j int) bool {
		si := severityRank(group[i].Severity)*10 + confidenceRank(group[i].Confidence)
		sj := severityRank(group[j].Severity)*10 + confidenceRank(group[j].Confidence)
		if si != sj {
			return si > sj // Higher rank first
		}
		// Tie-break by description length (more detail is better).
		return len(group[i].Description) > len(group[j].Description)
	})

	primary := group[0]

	// Count unique scanner attempts that reported this finding.
	attemptSet := make(map[string]bool)
	for _, f := range group {
		key := fmt.Sprintf("%s:%d", f.ChunkID, f.AttemptIndex)
		attemptSet[key] = true
	}

	// Find the min start line and max end line across all merged findings.
	minStart := primary.StartLine
	maxEnd := primary.EndLine
	for _, f := range group[1:] {
		if f.StartLine < minStart {
			minStart = f.StartLine
		}
		if f.EndLine > maxEnd {
			maxEnd = f.EndLine
		}
	}

	return DedupedFinding{
		FilePath:       primary.FilePath,
		StartLine:      minStart,
		EndLine:        maxEnd,
		Title:          primary.Title,
		Description:    primary.Description,
		Severity:       primary.Severity,
		CWE:            primary.CWE,
		Confidence:     primary.Confidence,
		VulnerableCode: primary.VulnerableCode,
		Remediation:    primary.Remediation,
		MergedFrom:     group,
		SourceCount:    len(attemptSet),
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// normalizeCWE normalizes CWE identifiers for comparison.
// Handles variations like "CWE-89", "cwe-89", "89", "CWE 89".
func normalizeCWE(cwe string) string {
	cwe = strings.TrimSpace(strings.ToUpper(cwe))
	if cwe == "" {
		return ""
	}
	// Strip "CWE-" or "CWE " prefix, then re-add canonical form.
	cwe = strings.TrimPrefix(cwe, "CWE-")
	cwe = strings.TrimPrefix(cwe, "CWE ")
	if cwe == "" {
		return ""
	}
	return "CWE-" + cwe
}

// severityRank returns a numeric rank for severity comparison.
// Higher rank = more severe.
func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

// confidenceRank returns a numeric rank for confidence comparison.
func confidenceRank(c string) int {
	switch strings.ToLower(c) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
