package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ── Pipeline ─────────────────────────────────────────────────────────────────

// Config holds all pipeline execution parameters.
type Config struct {
	// MaxConcurrent is the maximum number of parallel subagent invocations.
	MaxConcurrent int

	// BestOfK is the default number of independent scanner attempts per chunk.
	BestOfK int

	// ChunkMaxLines is the maximum number of lines per code chunk.
	ChunkMaxLines int

	// ScannerFn is the function that invokes a scanner subagent for a chunk.
	// It receives the chunk's scanner prompt and returns the raw findings.
	// The pipeline calls this function K times per chunk (best-of-K).
	ScannerFn func(ctx context.Context, prompt string) ([]RawFinding, error)

	// ReviewerFn is the function that invokes the reviewer subagent.
	// It receives deduplicated findings and returns confirmed/rejected findings.
	ReviewerFn func(ctx context.Context, findings []DedupedFinding) ([]ConfirmedFinding, error)

	// KnowledgeInjectionFn returns the knowledge base text to inject into a
	// scanner prompt, given the chunk's language, framework, risk level, etc.
	// If nil, no knowledge injection occurs.
	KnowledgeInjectionFn func(chunk Chunk) string
}

// Pipeline coordinates all scan phases from chunking through exploit writing.
type Pipeline struct {
	config    Config
	chunker   *Chunker
	dedup     *Deduplicator

	// Work queue semaphore for concurrency control.
	sem chan struct{}
}

// New creates a new Pipeline with the given configuration.
func New(cfg Config) *Pipeline {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.BestOfK <= 0 {
		cfg.BestOfK = 3
	}
	if cfg.ChunkMaxLines <= 0 {
		cfg.ChunkMaxLines = 150
	}

	return &Pipeline{
		config:  cfg,
		chunker: NewChunker(cfg.ChunkMaxLines, cfg.BestOfK),
		dedup:   NewDeduplicator(),
		sem:     make(chan struct{}, cfg.MaxConcurrent),
	}
}

// FileInput represents a source file to be scanned.
type FileInput struct {
	Path    string // Relative path from workspace root
	Content string // Full file content
}

// Run executes the full scan pipeline on the given files.
// Returns the pipeline result with confirmed and rejected findings.
func (p *Pipeline) Run(ctx context.Context, files []FileInput) (*PipelineResult, error) {
	start := time.Now()
	logger := slog.Default()

	result := &PipelineResult{
		Stats: PipelineStats{
			TotalFiles: len(files),
		},
	}

	// ── Phase 1: Chunk all files ──────────────────────────────────────────
	logger.Info("pipeline: phase 1 — chunking files", "files", len(files))

	var allChunks []Chunk
	for _, f := range files {
		chunks := p.chunker.ChunkFile(f.Path, f.Content)
		allChunks = append(allChunks, chunks...)
	}

	result.Stats.TotalChunks = len(allChunks)
	logger.Info("pipeline: chunking complete", "chunks", len(allChunks))

	if len(allChunks) == 0 {
		result.Stats.Duration = time.Since(start).String()
		return result, nil
	}

	// ── Phase 2: Parallel chunk scanning (best-of-K) ──────────────────────
	logger.Info("pipeline: phase 2 — parallel scanning",
		"chunks", len(allChunks),
		"max_concurrent", p.config.MaxConcurrent,
	)

	rawFindings, scannerCalls := p.scanChunks(ctx, allChunks)

	result.Stats.TotalScannerCalls = scannerCalls
	result.Stats.RawFindingsCount = len(rawFindings)
	logger.Info("pipeline: scanning complete",
		"raw_findings", len(rawFindings),
		"scanner_calls", scannerCalls,
	)

	if len(rawFindings) == 0 {
		result.Stats.Duration = time.Since(start).String()
		return result, nil
	}

	// ── Phase 3: Deterministic dedup ──────────────────────────────────────
	logger.Info("pipeline: phase 3 — deduplication", "raw_findings", len(rawFindings))

	dedupedFindings := p.dedup.Dedup(rawFindings)

	result.Stats.DedupedFindingsCount = len(dedupedFindings)
	logger.Info("pipeline: dedup complete",
		"deduped_findings", len(dedupedFindings),
		"reduction", fmt.Sprintf("%.0f%%", (1-float64(len(dedupedFindings))/float64(len(rawFindings)))*100),
	)

	// ── Phase 4: LLM reviewer ────────────────────────────────────────────
	if p.config.ReviewerFn != nil && len(dedupedFindings) > 0 {
		logger.Info("pipeline: phase 4 — LLM review", "findings", len(dedupedFindings))

		confirmed, err := p.config.ReviewerFn(ctx, dedupedFindings)
		if err != nil {
			logger.Error("pipeline: reviewer failed, using all deduped findings as confirmed",
				"error", err,
			)
			// Fallback: treat all deduped findings as confirmed.
			for _, df := range dedupedFindings {
				result.Confirmed = append(result.Confirmed, ConfirmedFinding{
					DedupedFinding:    df,
					ReviewerVerdict:   "unreviewed",
					ReviewerReasoning: "reviewer unavailable: " + err.Error(),
					AdjustedSeverity:  df.Severity,
					Fingerprint:       fmt.Sprintf("aegis-%s-%d-%s", df.FilePath, df.StartLine, df.CWE),
				})
			}
		} else {
			for _, cf := range confirmed {
				switch cf.ReviewerVerdict {
				case "confirmed", "likely":
					result.Confirmed = append(result.Confirmed, cf)
				default:
					result.Rejected = append(result.Rejected, cf)
				}
			}
		}
	} else {
		// No reviewer — treat all deduped findings as confirmed.
		for _, df := range dedupedFindings {
			result.Confirmed = append(result.Confirmed, ConfirmedFinding{
				DedupedFinding:    df,
				ReviewerVerdict:   "unreviewed",
				ReviewerReasoning: "no reviewer configured",
				AdjustedSeverity:  df.Severity,
				Fingerprint:       fmt.Sprintf("aegis-%s-%d-%s", df.FilePath, df.StartLine, df.CWE),
			})
		}
	}

	result.Stats.ConfirmedCount = len(result.Confirmed)
	result.Stats.FalsePositiveCount = len(result.Rejected)
	result.Stats.Duration = time.Since(start).String()

	logger.Info("pipeline: complete",
		"confirmed", len(result.Confirmed),
		"rejected", len(result.Rejected),
		"duration", result.Stats.Duration,
	)

	return result, nil
}

// scanChunks dispatches all chunks for parallel scanning with best-of-K retries.
// It respects the MaxConcurrent limit via a semaphore.
func (p *Pipeline) scanChunks(ctx context.Context, chunks []Chunk) ([]RawFinding, int) {
	if p.config.ScannerFn == nil {
		return nil, 0
	}

	type scanResult struct {
		findings []RawFinding
		err      error
	}

	totalCalls := 0
	for _, chunk := range chunks {
		totalCalls += chunk.BestOfK
	}

	resultsChan := make(chan scanResult, totalCalls)

	var wg sync.WaitGroup

	for _, chunk := range chunks {
		for attempt := 0; attempt < chunk.BestOfK; attempt++ {
			wg.Add(1)

			go func(ch Chunk, attemptIdx int) {
				defer wg.Done()

				// Acquire semaphore slot.
				select {
				case p.sem <- struct{}{}:
					defer func() { <-p.sem }()
				case <-ctx.Done():
					resultsChan <- scanResult{err: ctx.Err()}
					return
				}

				// Build prompt with knowledge injection.
				knowledgeText := ""
				if p.config.KnowledgeInjectionFn != nil {
					knowledgeText = p.config.KnowledgeInjectionFn(ch)
				}
				prompt := ch.BuildScannerPrompt(knowledgeText, len(chunks))

				// Invoke scanner subagent.
				findings, err := p.config.ScannerFn(ctx, prompt)
				if err != nil {
					slog.Warn("scanner subagent failed",
						"chunk", ch.ID,
						"attempt", attemptIdx,
						"error", err,
					)
					resultsChan <- scanResult{err: err}
					return
				}

				// Tag findings with chunk and attempt metadata.
				for i := range findings {
					findings[i].ChunkID = ch.ID
					findings[i].AttemptIndex = attemptIdx
					if findings[i].FilePath == "" {
						findings[i].FilePath = ch.FilePath
					}
				}

				resultsChan <- scanResult{findings: findings}
			}(chunk, attempt)
		}
	}

	// Wait for all goroutines and close channel.
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect all results.
	var allFindings []RawFinding
	for res := range resultsChan {
		if res.err != nil {
			continue // Errors logged above; other attempts may succeed.
		}
		allFindings = append(allFindings, res.findings...)
	}

	return allFindings, totalCalls
}
