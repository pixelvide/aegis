package knowledge

import (
	"fmt"
	"strings"
)

// ── Selector ─────────────────────────────────────────────────────────────────

// SelectionCriteria describes what the selector should look for
// when choosing patterns for a specific code chunk.
type SelectionCriteria struct {
	Language  string // e.g., "go", "python", "c"
	Framework string // e.g., "laravel", "express" (optional)
	RiskLevel string // "high", "medium", "low"
	FuncName  string // Function name for keyword matching (optional)
	Content   string // Code content for keyword matching (optional)
}

// MaxPatternsPerChunk is the default maximum number of patterns injected
// into a single scanner subagent prompt. Keeps context small and focused.
const MaxPatternsPerChunk = 10

// scoredPattern pairs a pattern with its relevance score for sorting.
type scoredPattern struct {
	pattern Pattern
	score   int
}

// Select returns the most relevant patterns for the given criteria.
// It scores each pattern based on language match, framework match,
// and keyword relevance, then returns the top N patterns.
func (kb *KnowledgeBase) Select(criteria SelectionCriteria) []Pattern {
	if len(kb.patterns) == 0 {
		return nil
	}

	// Collect candidate pattern indices by language.
	candidateSet := make(map[int]bool)
	if indices, ok := kb.byLanguage[criteria.Language]; ok {
		for _, idx := range indices {
			candidateSet[idx] = true
		}
	}
	// Also include language-agnostic patterns (empty languages list).
	for idx, p := range kb.patterns {
		if len(p.Languages) == 0 {
			candidateSet[idx] = true
		}
	}

	if len(candidateSet) == 0 {
		return nil
	}

	// Score each candidate.
	var candidates []scoredPattern

	lowerFunc := strings.ToLower(criteria.FuncName)
	lowerContent := strings.ToLower(criteria.Content)

	for idx := range candidateSet {
		p := kb.patterns[idx]
		score := scorePattern(p, criteria, lowerFunc, lowerContent)
		if score > 0 {
			candidates = append(candidates, scoredPattern{pattern: p, score: score})
		}
	}

	// Sort by score descending.
	sortScoredPatterns(candidates)

	// Take top N.
	maxN := MaxPatternsPerChunk
	if len(candidates) < maxN {
		maxN = len(candidates)
	}

	result := make([]Pattern, maxN)
	for i := 0; i < maxN; i++ {
		result[i] = candidates[i].pattern
	}
	return result
}

// FormatForPrompt converts selected patterns into a text block suitable
// for injection into a scanner subagent prompt.
func FormatForPrompt(patterns []Pattern) string {
	if len(patterns) == 0 {
		return ""
	}

	var b strings.Builder

	for i, p := range patterns {
		b.WriteString(fmt.Sprintf("Pattern %d: %s [%s] (%s)\n", i+1, p.Description, p.CWE, p.Severity))

		if len(p.DangerousSinks) > 0 {
			b.WriteString("  Dangerous sinks: ")
			b.WriteString(strings.Join(p.DangerousSinks, ", "))
			b.WriteString("\n")
		}
		if len(p.Indicators) > 0 {
			b.WriteString("  Indicators: ")
			b.WriteString(strings.Join(p.Indicators, ", "))
			b.WriteString("\n")
		}
		if len(p.TriggerConditions) > 0 {
			b.WriteString("  Trigger conditions:\n")
			for _, tc := range p.TriggerConditions {
				b.WriteString(fmt.Sprintf("    - %s\n", tc))
			}
		}
		if len(p.SafeAlternatives) > 0 {
			b.WriteString("  Safe alternatives: ")
			b.WriteString(strings.Join(p.SafeAlternatives, ", "))
			b.WriteString("\n")
		}
		if len(p.FalsePositiveSignals) > 0 {
			b.WriteString("  False positive signals: ")
			b.WriteString(strings.Join(p.FalsePositiveSignals, ", "))
			b.WriteString("\n")
		}
		if len(p.RealWorldExamples) > 0 {
			b.WriteString("  Real-world examples:\n")
			for _, ex := range p.RealWorldExamples {
				b.WriteString(fmt.Sprintf("    - %s\n", ex))
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ── Internal Scoring ─────────────────────────────────────────────────────────

// scorePattern assigns a relevance score to a pattern for the given criteria.
func scorePattern(p Pattern, criteria SelectionCriteria, lowerFunc, lowerContent string) int {
	score := 1 // Base score for language match.

	// Framework match bonus.
	if criteria.Framework != "" {
		for _, fw := range p.Frameworks {
			if strings.EqualFold(fw, criteria.Framework) {
				score += 5
				break
			}
		}
	}

	// Category bonus for high-risk chunks.
	if criteria.RiskLevel == "high" {
		switch p.Category {
		case "web", "systems", "crypto":
			score += 2
		}
	}

	// Keyword match bonus: check if any dangerous sinks or indicators
	// appear in the function name or code content.
	combined := lowerFunc + " " + lowerContent

	for _, sink := range p.DangerousSinks {
		if strings.Contains(combined, strings.ToLower(sink)) {
			score += 3
			break
		}
	}
	for _, indicator := range p.Indicators {
		if strings.Contains(combined, strings.ToLower(indicator)) {
			score += 3
			break
		}
	}

	// CWE-specific bonuses for common high-impact categories.
	switch p.CWE {
	case "CWE-89", "CWE-78", "CWE-79", "CWE-120", "CWE-416":
		score += 1 // Slight boost for commonly exploitable CWEs.
	}

	return score
}

// sortScoredPatterns performs a simple insertion sort (sufficient for <100 items).
func sortScoredPatterns(items []scoredPattern) {
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && items[j].score < key.score {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}

