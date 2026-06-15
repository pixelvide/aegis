// Package knowledge provides the vulnerability knowledge base for scanner
// prompt injection. It loads structured YAML patterns (organized by category)
// and selects the most relevant patterns for each code chunk based on
// language, framework, and function purpose.
package knowledge

import (
	"embed"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed patterns/**/*.yml
var patternsFS embed.FS

// Pattern represents a single vulnerability pattern in the knowledge base.
type Pattern struct {
	ID          string   `yaml:"id"`
	Source      string   `yaml:"source"`                // e.g., "sharingan-prompt-v1", "cwe-top25-v1"
	Layer       int      `yaml:"layer"`                 // 1 = from prompts, 2 = CWE gap fill, 3 = organic
	Languages   []string `yaml:"languages"`             // e.g., ["go", "python"]
	Frameworks  []string `yaml:"frameworks,omitempty"`   // e.g., ["laravel", "express"]
	Description string   `yaml:"description"`
	Category    string   `yaml:"-"` // Set from directory name (web, systems, crypto, supply_chain)

	// Indicators — what to look for in code.
	DangerousSinks    []string `yaml:"dangerous_sinks,omitempty"`
	Indicators        []string `yaml:"indicators,omitempty"`
	TriggerConditions []string `yaml:"trigger_conditions,omitempty"`

	// Context for reducing false positives.
	SafeAlternatives     []string `yaml:"safe_alternatives,omitempty"`
	FalsePositiveSignals []string `yaml:"false_positive_signals,omitempty"`

	// Classification.
	CWE      string `yaml:"cwe"`
	Severity string `yaml:"severity"`

	// Real-world references.
	RealWorldExamples []string `yaml:"real_world_examples,omitempty"`
}

// KnowledgeBase holds all loaded vulnerability patterns, indexed for fast
// lookup by language and category.
type KnowledgeBase struct {
	patterns []Pattern

	// Indexes for fast selection.
	byLanguage  map[string][]int // language -> pattern indices
	byCategory  map[string][]int // category -> pattern indices
	byFramework map[string][]int // framework -> pattern indices
}

// Load reads all embedded YAML pattern files and builds the knowledge base.
// This should be called once at agent startup.
func Load() (*KnowledgeBase, error) {
	kb := &KnowledgeBase{
		byLanguage:  make(map[string][]int),
		byCategory:  make(map[string][]int),
		byFramework: make(map[string][]int),
	}

	entries, err := patternsFS.ReadDir("patterns")
	if err != nil {
		return nil, fmt.Errorf("read patterns directory: %w", err)
	}

	for _, categoryDir := range entries {
		if !categoryDir.IsDir() {
			continue
		}
		category := categoryDir.Name()

		files, err := patternsFS.ReadDir(filepath.Join("patterns", category))
		if err != nil {
			slog.Warn("skipping category directory", "category", category, "error", err)
			continue
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".yml") {
				continue
			}

			data, err := patternsFS.ReadFile(filepath.Join("patterns", category, f.Name()))
			if err != nil {
				slog.Warn("skipping pattern file", "file", f.Name(), "error", err)
				continue
			}

			var patterns []Pattern
			if err := yaml.Unmarshal(data, &patterns); err != nil {
				slog.Warn("skipping malformed pattern file", "file", f.Name(), "error", err)
				continue
			}

			for i := range patterns {
				patterns[i].Category = category
			}

			kb.addPatterns(patterns)
		}
	}

	slog.Info("knowledge base loaded", "patterns", len(kb.patterns))
	return kb, nil
}

// addPatterns adds patterns to the knowledge base and updates indexes.
func (kb *KnowledgeBase) addPatterns(patterns []Pattern) {
	for _, p := range patterns {
		idx := len(kb.patterns)
		kb.patterns = append(kb.patterns, p)

		// Index by language.
		for _, lang := range p.Languages {
			kb.byLanguage[lang] = append(kb.byLanguage[lang], idx)
		}

		// Index by category.
		kb.byCategory[p.Category] = append(kb.byCategory[p.Category], idx)

		// Index by framework.
		for _, fw := range p.Frameworks {
			kb.byFramework[fw] = append(kb.byFramework[fw], idx)
		}
	}
}

// TotalPatterns returns the number of patterns in the knowledge base.
func (kb *KnowledgeBase) TotalPatterns() int {
	return len(kb.patterns)
}

// PatternsByLayer returns counts per layer for reporting.
func (kb *KnowledgeBase) PatternsByLayer() map[int]int {
	counts := make(map[int]int)
	for _, p := range kb.patterns {
		counts[p.Layer]++
	}
	return counts
}
