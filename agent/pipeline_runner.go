package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pixelvide/aegis/agent/knowledge"
	"github.com/pixelvide/aegis/agent/pipeline"
	"github.com/pixelvide/localharness/adk"
)

// ── Pipeline Runner ──────────────────────────────────────────────────────────

// PipelineRunner bridges the pipeline engine with the ADK agent.
// It creates scanner and reviewer functions that delegate to subagent
// invocations, manages the knowledge base, and coordinates the scan.
type PipelineRunner struct {
	agent    *adk.Agent
	config   AegisConfig
	reporter *Reporter
	kb       *knowledge.KnowledgeBase
}

// NewPipelineRunner creates a runner with the given agent and config.
func NewPipelineRunner(agent *adk.Agent, config AegisConfig, reporter *Reporter) (*PipelineRunner, error) {
	// Load knowledge base (embedded YAML patterns).
	kb, err := knowledge.Load()
	if err != nil {
		slog.Warn("knowledge base load failed, continuing without KB", "error", err)
		// kb will be nil — pipeline runs without knowledge injection.
	}

	return &PipelineRunner{
		agent:    agent,
		config:   config,
		reporter: reporter,
		kb:       kb,
	}, nil
}

// Run executes the full pipeline: discover files → chunk → scan → dedup → review → report.
func (pr *PipelineRunner) Run(ctx context.Context, prompt string) error {
	scanCfg := pr.config.ResolveScan()
	roles := pr.config.ResolveRoles()

	slog.Info("pipeline: starting",
		"max_concurrent", scanCfg.MaxConcurrent,
		"best_of_k", scanCfg.BestOfK,
		"chunk_max_lines", scanCfg.ChunkMaxLines,
		"scanner_model", roles.Scanner.Model,
		"reviewer_model", roles.Reviewer.Model,
	)

	// Discover source files to scan.
	files, err := pr.discoverFiles(ctx)
	if err != nil {
		return fmt.Errorf("file discovery failed: %w", err)
	}

	if len(files) == 0 {
		slog.Info("pipeline: no source files found")
		return nil
	}
	slog.Info("pipeline: discovered source files", "count", len(files))

	// Build pipeline config with scanner and reviewer functions.
	pipelineCfg := pipeline.Config{
		MaxConcurrent: scanCfg.MaxConcurrent,
		BestOfK:       scanCfg.BestOfK,
		ChunkMaxLines: scanCfg.ChunkMaxLines,
		ScannerFn:     pr.makeScannerFn(roles.Scanner),
		ReviewerFn:    pr.makeReviewerFn(roles.Reviewer),
	}

	if pr.kb != nil {
		pipelineCfg.KnowledgeInjectionFn = pr.makeKnowledgeInjectionFn()
	}

	// Run the pipeline.
	p := pipeline.New(pipelineCfg)
	result, err := p.Run(ctx, files)
	if err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Push confirmed findings to the reporter.
	for _, cf := range result.Confirmed {
		pr.reportFinding(ctx, cf)
	}

	// Print summary.
	pr.printSummary(result)

	return nil
}

// ── File Discovery ───────────────────────────────────────────────────────────

// discoverFiles uses a subtask to list and read source files from the workspace.
func (pr *PipelineRunner) discoverFiles(ctx context.Context) ([]pipeline.FileInput, error) {
	wsDir := pr.config.Workspace
	if wsDir == "" {
		wsDir, _ = os.Getwd()
	}

	// Walk directory for source files, respecting .gitignore-style patterns.
	var files []pipeline.FileInput
	supportedExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".jsx": true,
		".ts": true, ".tsx": true, ".java": true, ".c": true,
		".h": true, ".cpp": true, ".cc": true, ".hpp": true,
		".rs": true, ".rb": true, ".php": true, ".cs": true,
		".swift": true, ".kt": true, ".scala": true, ".sol": true,
	}

	skipDirs := map[string]bool{
		"node_modules": true, "vendor": true, ".git": true,
		"dist": true, "build": true, "__pycache__": true,
		".next": true, ".aegis": true, "coverage": true,
		"target": true, "bin": true, "obj": true,
	}

	err := filepath.Walk(wsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip unreadable paths.
		}

		// Skip hidden and excluded directories.
		if info.IsDir() {
			base := filepath.Base(path)
			if skipDirs[base] || (len(base) > 1 && base[0] == '.') {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process supported source file extensions.
		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExts[ext] {
			return nil
		}

		// Skip large files (> 500KB — likely generated or vendored).
		if info.Size() > 500*1024 {
			return nil
		}

		// Read file content.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip unreadable files.
		}

		relPath, err := filepath.Rel(wsDir, path)
		if err != nil {
			relPath = path
		}

		files = append(files, pipeline.FileInput{
			Path:    relPath,
			Content: string(data),
		})

		return nil
	})

	return files, err
}

// ── Scanner Function ─────────────────────────────────────────────────────────

// makeScannerFn returns a function that invokes a chunk-scanner subtask.
func (pr *PipelineRunner) makeScannerFn(role RoleConfig) func(ctx context.Context, prompt string) ([]pipeline.RawFinding, error) {
	return func(ctx context.Context, prompt string) ([]pipeline.RawFinding, error) {
		result, err := pr.agent.RunSubtask(ctx, adk.SubtaskConfig{
			Prompt:       prompt,
			SystemPrompt: chunkScannerPrompt,
			ReadOnly:     adk.Bool(true),
			Model:        role.Model,
			Timeout:      60 * time.Second, // Scanner should return JSON immediately, not browse files.
		})
		if err != nil {
			return nil, err
		}

		// Parse JSON findings from the subtask response.
		return parseScannerResponse(result.Text)
	}
}

// parseScannerResponse extracts findings from the scanner's JSON output.
func parseScannerResponse(text string) ([]pipeline.RawFinding, error) {
	// Extract JSON from the response (might be wrapped in markdown code blocks).
	jsonStr := extractJSON(text)
	if jsonStr == "" {
		return nil, nil // No findings.
	}

	var response struct {
		Findings []struct {
			Title          string `json:"title"`
			Description    string `json:"description"`
			Severity       string `json:"severity"`
			CWE            string `json:"cwe"`
			Confidence     string `json:"confidence"`
			StartLine      int    `json:"start_line"`
			EndLine        int    `json:"end_line"`
			VulnerableCode string `json:"vulnerable_code"`
			Remediation    string `json:"remediation"`
		} `json:"findings"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		slog.Warn("scanner: failed to parse JSON response", "error", err)
		return nil, nil
	}

	var findings []pipeline.RawFinding
	for _, f := range response.Findings {
		if f.Title == "" {
			continue
		}
		findings = append(findings, pipeline.RawFinding{
			Title:          f.Title,
			Description:    f.Description,
			Severity:       f.Severity,
			CWE:            f.CWE,
			Confidence:     f.Confidence,
			StartLine:      f.StartLine,
			EndLine:        f.EndLine,
			VulnerableCode: f.VulnerableCode,
			Remediation:    f.Remediation,
		})
	}

	return findings, nil
}

// ── Reviewer Function ────────────────────────────────────────────────────────

// makeReviewerFn returns a function that invokes a finding-reviewer subtask.
func (pr *PipelineRunner) makeReviewerFn(role RoleConfig) func(ctx context.Context, findings []pipeline.DedupedFinding) ([]pipeline.ConfirmedFinding, error) {
	return func(ctx context.Context, findings []pipeline.DedupedFinding) ([]pipeline.ConfirmedFinding, error) {
		// Build reviewer prompt with all findings.
		prompt := buildReviewerPrompt(findings)

		result, err := pr.agent.RunSubtask(ctx, adk.SubtaskConfig{
			Prompt:       prompt,
			SystemPrompt: findingReviewerPrompt,
			ReadOnly:     adk.Bool(true),
			Model:        role.Model,
		})
		if err != nil {
			return nil, err
		}

		// Parse reviewer response.
		return parseReviewerResponse(result.Text, findings)
	}
}

// buildReviewerPrompt creates the prompt for the finding reviewer.
func buildReviewerPrompt(findings []pipeline.DedupedFinding) string {
	var b strings.Builder
	b.WriteString("Review the following deduplicated security findings.\n")
	b.WriteString("For each finding, validate exploitability, re-rank severity, and identify false positives.\n\n")

	for _, f := range findings {
		b.WriteString(fmt.Sprintf("--- %s ---\n", f.ID))
		b.WriteString(fmt.Sprintf("Title: %s\n", f.Title))
		b.WriteString(fmt.Sprintf("File: %s (lines %d-%d)\n", f.FilePath, f.StartLine, f.EndLine))
		b.WriteString(fmt.Sprintf("Severity: %s | CWE: %s | Confidence: %s\n", f.Severity, f.CWE, f.Confidence))
		b.WriteString(fmt.Sprintf("Source Count: %d (independent scanners that found this)\n", f.SourceCount))
		b.WriteString(fmt.Sprintf("Description: %s\n", f.Description))
		if f.VulnerableCode != "" {
			b.WriteString(fmt.Sprintf("Code: %s\n", f.VulnerableCode))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// parseReviewerResponse extracts review verdicts from the reviewer's JSON output.
func parseReviewerResponse(text string, originals []pipeline.DedupedFinding) ([]pipeline.ConfirmedFinding, error) {
	jsonStr := extractJSON(text)
	if jsonStr == "" {
		// Reviewer didn't return JSON — treat all findings as confirmed.
		var confirmed []pipeline.ConfirmedFinding
		for _, f := range originals {
			confirmed = append(confirmed, pipeline.ConfirmedFinding{
				DedupedFinding:    f,
				ReviewerVerdict:   "unreviewed",
				ReviewerReasoning: "reviewer returned non-JSON response",
				AdjustedSeverity:  f.Severity,
				Fingerprint:       fmt.Sprintf("aegis-%s-%d-%s", f.FilePath, f.StartLine, f.CWE),
			})
		}
		return confirmed, nil
	}

	var response struct {
		Reviewed []struct {
			ID               string   `json:"id"`
			Verdict          string   `json:"verdict"`
			Reasoning        string   `json:"reasoning"`
			AdjustedSeverity string   `json:"adjusted_severity"`
			ChainIDs         []string `json:"chain_ids"`
			ChainDescription string   `json:"chain_description"`
		} `json:"reviewed"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &response); err != nil {
		slog.Warn("reviewer: failed to parse JSON response", "error", err)
		// Fall back to treating all as confirmed.
		var confirmed []pipeline.ConfirmedFinding
		for _, f := range originals {
			confirmed = append(confirmed, pipeline.ConfirmedFinding{
				DedupedFinding:    f,
				ReviewerVerdict:   "unreviewed",
				ReviewerReasoning: "reviewer JSON parse failed",
				AdjustedSeverity:  f.Severity,
				Fingerprint:       fmt.Sprintf("aegis-%s-%d-%s", f.FilePath, f.StartLine, f.CWE),
			})
		}
		return confirmed, nil
	}

	// Build a lookup map for originals.
	origMap := make(map[string]pipeline.DedupedFinding)
	for _, f := range originals {
		origMap[f.ID] = f
	}

	// Build a lookup map for reviewer verdicts.
	verdictMap := make(map[string]struct {
		Verdict          string
		Reasoning        string
		AdjustedSeverity string
		ChainIDs         []string
		ChainDescription string
	})
	for _, rv := range response.Reviewed {
		verdictMap[rv.ID] = struct {
			Verdict          string
			Reasoning        string
			AdjustedSeverity string
			ChainIDs         []string
			ChainDescription string
		}{rv.Verdict, rv.Reasoning, rv.AdjustedSeverity, rv.ChainIDs, rv.ChainDescription}
	}

	var confirmed []pipeline.ConfirmedFinding
	for _, f := range originals {
		cf := pipeline.ConfirmedFinding{
			DedupedFinding: f,
			Fingerprint:    fmt.Sprintf("aegis-%s-%d-%s", f.FilePath, f.StartLine, f.CWE),
		}

		if rv, ok := verdictMap[f.ID]; ok {
			cf.ReviewerVerdict = rv.Verdict
			cf.ReviewerReasoning = rv.Reasoning
			cf.AdjustedSeverity = rv.AdjustedSeverity
			cf.ChainIDs = rv.ChainIDs
			cf.ChainDescription = rv.ChainDescription
		} else {
			cf.ReviewerVerdict = "unreviewed"
			cf.ReviewerReasoning = "reviewer did not assess this finding"
			cf.AdjustedSeverity = f.Severity
		}

		if cf.AdjustedSeverity == "" {
			cf.AdjustedSeverity = f.Severity
		}

		confirmed = append(confirmed, cf)
	}

	return confirmed, nil
}

// ── Knowledge Injection ──────────────────────────────────────────────────────

// makeKnowledgeInjectionFn returns a function that selects and formats
// relevant KB patterns for a given chunk.
func (pr *PipelineRunner) makeKnowledgeInjectionFn() func(chunk pipeline.Chunk) string {
	return func(chunk pipeline.Chunk) string {
		if pr.kb == nil {
			return ""
		}

		patterns := pr.kb.Select(knowledge.SelectionCriteria{
			Language:  chunk.Language,
			Framework: chunk.Framework,
			RiskLevel: chunk.RiskLevel,
			FuncName:  chunk.FunctionName,
			Content:   chunk.Content,
		})

		return knowledge.FormatForPrompt(patterns)
	}
}

// ── Finding Reporting ────────────────────────────────────────────────────────

// reportFinding pushes a confirmed finding to the reporter.
func (pr *PipelineRunner) reportFinding(_ context.Context, cf pipeline.ConfirmedFinding) {
	// Skip false positives and unlikely findings.
	switch cf.ReviewerVerdict {
	case "false_positive", "unlikely":
		return
	}

	args := map[string]any{
		"fingerprint": cf.Fingerprint,
		"title":       cf.Title,
		"severity":    cf.AdjustedSeverity,
		"description": cf.Description,
		"file":        cf.FilePath,
		"line":        float64(cf.StartLine),
		"cwe":         cf.CWE,
	}

	if _, err := pr.reporter.handleReportFinding(context.Background(), args); err != nil {
		slog.Warn("pipeline: failed to report finding", "title", cf.Title, "error", err)
	}
}

// ── Summary ──────────────────────────────────────────────────────────────────

// printSummary prints the pipeline execution summary.
func (pr *PipelineRunner) printSummary(result *pipeline.PipelineResult) {
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("🔬 Pipeline Summary\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Files scanned:     %d\n", result.Stats.TotalFiles)
	fmt.Printf("Chunks created:    %d\n", result.Stats.TotalChunks)
	fmt.Printf("Scanner calls:     %d\n", result.Stats.TotalScannerCalls)
	fmt.Printf("Raw findings:      %d\n", result.Stats.RawFindingsCount)
	fmt.Printf("After dedup:       %d\n", result.Stats.DedupedFindingsCount)
	fmt.Printf("Confirmed:         %d\n", result.Stats.ConfirmedCount)
	fmt.Printf("False positives:   %d\n", result.Stats.FalsePositiveCount)
	fmt.Printf("Duration:          %s\n", result.Stats.Duration)
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if len(result.Confirmed) > 0 {
		fmt.Printf("\n📋 Confirmed Findings:\n")
		for i, cf := range result.Confirmed {
			verdict := cf.ReviewerVerdict
			if verdict == "" {
				verdict = "unreviewed"
			}
			fmt.Printf("  %d. [%s] %s — %s:%d (%s, verdict: %s)\n",
				i+1, strings.ToUpper(cf.AdjustedSeverity), cf.Title,
				cf.FilePath, cf.StartLine, cf.CWE, verdict)
		}
	}
}

// ── JSON Extraction Helper ───────────────────────────────────────────────────

// extractJSON finds the first JSON object in a text response.
// Handles JSON wrapped in markdown code blocks (```json ... ```).
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// Try to find JSON in code blocks first.
	if idx := strings.Index(text, "```json"); idx != -1 {
		start := idx + 7
		end := strings.Index(text[start:], "```")
		if end != -1 {
			return strings.TrimSpace(text[start : start+end])
		}
	}
	if idx := strings.Index(text, "```"); idx != -1 {
		start := idx + 3
		// Skip language identifier if on the same line.
		if nl := strings.IndexByte(text[start:], '\n'); nl != -1 {
			start += nl + 1
		}
		end := strings.Index(text[start:], "```")
		if end != -1 {
			candidate := strings.TrimSpace(text[start : start+end])
			if len(candidate) > 0 && candidate[0] == '{' {
				return candidate
			}
		}
	}

	// Try to find a raw JSON object.
	if braceIdx := strings.IndexByte(text, '{'); braceIdx != -1 {
		// Find the matching closing brace.
		depth := 0
		for i := braceIdx; i < len(text); i++ {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return text[braceIdx : i+1]
				}
			}
		}
	}

	return ""
}
