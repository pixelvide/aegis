// Package pipeline implements the multi-phase scan pipeline for Aegis.
//
// The pipeline coordinates six phases:
//   - Phase 0: Static analysis (semgrep, grep, dependency scanners)
//   - Phase 1: Orchestration (chunking, risk classification, assignment)
//   - Phase 2: Parallel chunk scanning (best-of-K subagents)
//   - Phase 3: Deterministic deduplication
//   - Phase 4: LLM-based review and validation
//   - Phase 5: Parallel exploit writing
package pipeline

// Chunk represents a slice of a source file assigned to a scanner subagent.
// Each chunk contains a contiguous block of code (typically one function or
// logical block) along with context about the file and surrounding code.
type Chunk struct {
	// ID is a unique identifier for this chunk, e.g. "chunk_012".
	ID string `json:"id"`

	// FilePath is the path to the source file (relative to workspace root).
	FilePath string `json:"file_path"`

	// StartLine is the 1-indexed first line of the chunk in the source file.
	StartLine int `json:"start_line"`

	// EndLine is the 1-indexed last line of the chunk in the source file.
	EndLine int `json:"end_line"`

	// FunctionName is the name of the function or method in this chunk,
	// if identifiable from the source code. May be empty for non-function chunks.
	FunctionName string `json:"function_name,omitempty"`

	// Content is the actual source code of the chunk.
	Content string `json:"content"`

	// Context contains surrounding function signatures, imports, and struct
	// definitions that help the scanner understand the chunk without reading
	// the full file. This is injected into the scanner prompt.
	Context string `json:"context,omitempty"`

	// Language is the programming language of the source file (e.g., "go", "python").
	Language string `json:"language"`

	// Framework is the detected framework, if any (e.g., "laravel", "express").
	Framework string `json:"framework,omitempty"`

	// RiskLevel classifies the chunk's audit priority: "high", "medium", or "low".
	// High-risk chunks (auth, crypto, input parsing) get more best-of-K attempts.
	RiskLevel string `json:"risk_level"`

	// BestOfK is the number of independent scanner attempts for this chunk.
	// Higher values increase finding coverage at the cost of more API calls.
	BestOfK int `json:"best_of_k"`
}

// RawFinding is a finding reported by a scanner subagent before deduplication.
// Multiple raw findings may describe the same vulnerability (from best-of-K
// retries or overlapping chunks).
type RawFinding struct {
	// ChunkID identifies which chunk produced this finding.
	ChunkID string `json:"chunk_id"`

	// AttemptIndex identifies which best-of-K attempt produced this finding (0-indexed).
	AttemptIndex int `json:"attempt_index"`

	// FilePath is the file where the vulnerability was found.
	FilePath string `json:"file_path"`

	// StartLine is the first line of the vulnerable code.
	StartLine int `json:"start_line"`

	// EndLine is the last line of the vulnerable code.
	EndLine int `json:"end_line"`

	// Title is a short description of the vulnerability.
	Title string `json:"title"`

	// Description is a detailed explanation of the vulnerability, including
	// the attack vector, impact, and evidence.
	Description string `json:"description"`

	// Severity is the raw severity from the scanner: "critical", "high", "medium", "low", "info".
	Severity string `json:"severity"`

	// CWE is the CWE identifier (e.g., "CWE-89").
	CWE string `json:"cwe,omitempty"`

	// Confidence is the scanner's self-assessed confidence: "high", "medium", "low".
	Confidence string `json:"confidence"`

	// VulnerableCode is the specific code snippet that is vulnerable.
	VulnerableCode string `json:"vulnerable_code,omitempty"`

	// Remediation describes how to fix the vulnerability.
	Remediation string `json:"remediation,omitempty"`

	// Source identifies where this finding came from (e.g., "semgrep",
	// "trivy", "llm-scanner"). Used to distinguish external tool findings
	// from LLM-generated findings in dedup and review.
	Source string `json:"source,omitempty"`
}

// DedupedFinding is a finding after deterministic deduplication.
// It merges multiple RawFindings that describe the same root cause.
type DedupedFinding struct {
	// ID is a stable identifier for this deduplicated finding.
	ID string `json:"id"`

	// FilePath is the file where the vulnerability was found.
	FilePath string `json:"file_path"`

	// StartLine is the first line of the vulnerable code.
	StartLine int `json:"start_line"`

	// EndLine is the last line of the vulnerable code.
	EndLine int `json:"end_line"`

	// Title is the best title from the merged raw findings.
	Title string `json:"title"`

	// Description is the most detailed description from the merged findings.
	Description string `json:"description"`

	// Severity is the highest severity from the merged findings.
	Severity string `json:"severity"`

	// CWE is the CWE identifier.
	CWE string `json:"cwe,omitempty"`

	// Confidence is the highest confidence from the merged findings.
	Confidence string `json:"confidence"`

	// VulnerableCode is the code snippet.
	VulnerableCode string `json:"vulnerable_code,omitempty"`

	// Remediation describes how to fix the vulnerability.
	Remediation string `json:"remediation,omitempty"`

	// MergedFrom tracks the raw findings that were merged into this one.
	MergedFrom []RawFinding `json:"merged_from"`

	// SourceCount is the number of independent scanner attempts that
	// reported this finding. Higher count = higher confidence.
	SourceCount int `json:"source_count"`
}

// ConfirmedFinding is a finding that has been validated by the LLM reviewer.
// The reviewer confirms reachability, re-ranks severity, and identifies
// attack chains across findings.
type ConfirmedFinding struct {
	DedupedFinding

	// ReviewerVerdict is the reviewer's assessment: "confirmed", "likely", "unlikely", "false_positive".
	ReviewerVerdict string `json:"reviewer_verdict"`

	// ReviewerReasoning explains why the reviewer confirmed or rejected the finding.
	ReviewerReasoning string `json:"reviewer_reasoning"`

	// AdjustedSeverity is the severity after the reviewer's re-ranking.
	// May differ from the original if the reviewer determined different exploitability.
	AdjustedSeverity string `json:"adjusted_severity"`

	// ChainIDs lists the IDs of other findings that chain with this one
	// to create a higher-severity attack path.
	ChainIDs []string `json:"chain_ids,omitempty"`

	// ChainDescription describes the combined attack chain, if any.
	ChainDescription string `json:"chain_description,omitempty"`

	// Fingerprint is the stable dedup key for the Aegis reporter.
	Fingerprint string `json:"fingerprint"`
}

// PipelineResult is the final output of a complete pipeline run.
type PipelineResult struct {
	// ScanID is the UUID v7 for this scan run.
	ScanID string `json:"scan_id"`

	// Confirmed contains all findings that passed the reviewer validation.
	Confirmed []ConfirmedFinding `json:"confirmed"`

	// Rejected contains findings the reviewer marked as false positives.
	Rejected []ConfirmedFinding `json:"rejected"`

	// Stats contains pipeline execution statistics.
	Stats PipelineStats `json:"stats"`
}

// PipelineStats tracks metrics for the pipeline execution.
type PipelineStats struct {
	// TotalFiles is the number of source files analyzed.
	TotalFiles int `json:"total_files"`

	// TotalChunks is the total number of chunks created.
	TotalChunks int `json:"total_chunks"`

	// TotalScannerCalls is the total number of scanner subagent invocations
	// (chunks × best-of-K).
	TotalScannerCalls int `json:"total_scanner_calls"`

	// RawFindingsCount is the number of findings before dedup.
	RawFindingsCount int `json:"raw_findings_count"`

	// DedupedFindingsCount is the number of findings after dedup.
	DedupedFindingsCount int `json:"deduped_findings_count"`

	// ConfirmedCount is the number of findings confirmed by the reviewer.
	ConfirmedCount int `json:"confirmed_count"`

	// FalsePositiveCount is the number of findings rejected by the reviewer.
	FalsePositiveCount int `json:"false_positive_count"`

	// Duration is the total wall-clock time for the pipeline run.
	Duration string `json:"duration"`
}
