package pipeline

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// ── Chunker ──────────────────────────────────────────────────────────────────

// Chunker splits source files into function-sized chunks for parallel scanning.
// It uses language-aware heuristics to identify function boundaries and creates
// chunks with surrounding context (imports, type signatures) for scanner quality.
type Chunker struct {
	// MaxLines is the maximum number of lines per chunk.
	MaxLines int

	// DefaultBestOfK is the default best-of-K retry count for chunks.
	DefaultBestOfK int
}

// NewChunker creates a Chunker with default settings.
func NewChunker(maxLines, defaultBestOfK int) *Chunker {
	if maxLines <= 0 {
		maxLines = 150
	}
	if defaultBestOfK <= 0 {
		defaultBestOfK = 3
	}
	return &Chunker{
		MaxLines:       maxLines,
		DefaultBestOfK: defaultBestOfK,
	}
}

// ChunkFile splits a source file into chunks. The filePath is relative to the
// workspace root. The content is the full file source code.
//
// Chunking strategy:
//  1. Detect language from file extension
//  2. Split on function/method boundaries using language-specific patterns
//  3. If a function exceeds MaxLines, split it into logical sub-blocks
//  4. Each chunk gets an imports/context header from the file preamble
func (c *Chunker) ChunkFile(filePath, content string) []Chunk {
	lang := detectLanguage(filePath)
	lines := strings.Split(content, "\n")

	if len(lines) <= c.MaxLines {
		// Small file — return as a single chunk.
		return []Chunk{{
			ID:        "chunk_000",
			FilePath:  filePath,
			StartLine: 1,
			EndLine:   len(lines),
			Content:   content,
			Language:  lang,
			RiskLevel: "medium",
			BestOfK:   c.DefaultBestOfK,
		}}
	}

	// Find function boundaries.
	boundaries := findFunctionBoundaries(lines, lang)
	if len(boundaries) == 0 {
		// No functions detected — fall back to fixed-size splitting.
		return c.fixedSizeChunks(filePath, lines, lang)
	}

	// Extract file preamble (imports, package declaration) as shared context.
	preamble := extractPreamble(lines, boundaries, lang)

	// Create chunks from function boundaries.
	var chunks []Chunk
	chunkIdx := 0

	for _, fn := range boundaries {
		fnLines := lines[fn.startLine-1 : fn.endLine]
		fnContent := strings.Join(fnLines, "\n")

		if len(fnLines) <= c.MaxLines {
			// Function fits in a single chunk.
			chunks = append(chunks, Chunk{
				ID:           fmt.Sprintf("chunk_%03d", chunkIdx),
				FilePath:     filePath,
				StartLine:    fn.startLine,
				EndLine:      fn.endLine,
				FunctionName: fn.name,
				Content:      fnContent,
				Context:      preamble,
				Language:     lang,
				RiskLevel:    classifyRisk(fn.name, fnContent),
				BestOfK:      c.DefaultBestOfK,
			})
			chunkIdx++
		} else {
			// Large function — split into sub-chunks.
			subChunks := c.splitLargeFunction(filePath, fn, fnLines, preamble, lang, &chunkIdx)
			chunks = append(chunks, subChunks...)
		}
	}

	// Handle code between functions (global scope, constants, etc.)
	// This catches any code that isn't inside a function boundary.
	interstitial := c.findInterstitialCode(filePath, lines, boundaries, preamble, lang, &chunkIdx)
	chunks = append(chunks, interstitial...)

	return chunks
}

// functionBoundary represents a detected function/method in the source code.
type functionBoundary struct {
	name      string
	startLine int // 1-indexed
	endLine   int // 1-indexed, inclusive
}

// findFunctionBoundaries detects function start/end lines using language-specific patterns.
func findFunctionBoundaries(lines []string, lang string) []functionBoundary {
	patterns := functionPatterns[lang]
	if patterns == nil {
		return nil
	}

	var boundaries []functionBoundary
	var current *functionBoundary
	braceDepth := 0

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		if current == nil {
			// Look for function start.
			for _, p := range patterns {
				if matches := p.FindStringSubmatch(trimmed); matches != nil {
					name := ""
					if len(matches) > 1 {
						name = matches[1]
					}
					current = &functionBoundary{
						name:      name,
						startLine: lineNum,
					}
					braceDepth = strings.Count(line, "{") - strings.Count(line, "}")
					break
				}
			}
		} else {
			// Track brace depth to find function end.
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 {
				current.endLine = lineNum
				boundaries = append(boundaries, *current)
				current = nil
				braceDepth = 0
			}
		}
	}

	// Handle unclosed function (last function in file).
	if current != nil {
		current.endLine = len(lines)
		boundaries = append(boundaries, *current)
	}

	return boundaries
}

// splitLargeFunction splits a function that exceeds MaxLines into sub-chunks
// at logical break points (blank lines, comment blocks).
func (c *Chunker) splitLargeFunction(filePath string, fn functionBoundary, fnLines []string, preamble, lang string, chunkIdx *int) []Chunk {
	var chunks []Chunk
	start := 0

	for start < len(fnLines) {
		end := start + c.MaxLines
		if end >= len(fnLines) {
			end = len(fnLines)
		} else {
			// Try to split at a blank line for cleaner breaks.
			for j := end; j > start+c.MaxLines/2; j-- {
				if strings.TrimSpace(fnLines[j]) == "" {
					end = j + 1
					break
				}
			}
		}

		chunkContent := strings.Join(fnLines[start:end], "\n")
		chunks = append(chunks, Chunk{
			ID:           fmt.Sprintf("chunk_%03d", *chunkIdx),
			FilePath:     filePath,
			StartLine:    fn.startLine + start,
			EndLine:      fn.startLine + end - 1,
			FunctionName: fn.name,
			Content:      chunkContent,
			Context:      preamble,
			Language:     lang,
			RiskLevel:    classifyRisk(fn.name, chunkContent),
			BestOfK:      c.DefaultBestOfK,
		})
		*chunkIdx++
		start = end
	}

	return chunks
}

// fixedSizeChunks splits a file without clear function boundaries into
// fixed-size chunks, breaking at blank lines when possible.
func (c *Chunker) fixedSizeChunks(filePath string, lines []string, lang string) []Chunk {
	var chunks []Chunk
	chunkIdx := 0
	start := 0

	for start < len(lines) {
		end := start + c.MaxLines
		if end >= len(lines) {
			end = len(lines)
		} else {
			// Try to split at a blank line.
			for j := end; j > start+c.MaxLines/2; j-- {
				if strings.TrimSpace(lines[j]) == "" {
					end = j + 1
					break
				}
			}
		}

		chunkContent := strings.Join(lines[start:end], "\n")
		chunks = append(chunks, Chunk{
			ID:        fmt.Sprintf("chunk_%03d", chunkIdx),
			FilePath:  filePath,
			StartLine: start + 1,
			EndLine:   end,
			Content:   chunkContent,
			Language:  lang,
			RiskLevel: "medium",
			BestOfK:   c.DefaultBestOfK,
		})
		chunkIdx++
		start = end
	}

	return chunks
}

// findInterstitialCode finds code that falls between function boundaries
// (global scope, constants, type definitions, etc.) and creates chunks for it.
func (c *Chunker) findInterstitialCode(filePath string, lines []string, boundaries []functionBoundary, preamble, lang string, chunkIdx *int) []Chunk {
	var chunks []Chunk

	// Build a set of lines covered by functions.
	covered := make(map[int]bool)
	for _, fn := range boundaries {
		for i := fn.startLine; i <= fn.endLine; i++ {
			covered[i] = true
		}
	}

	// Find contiguous uncovered ranges.
	var uncoveredStart int
	inUncovered := false

	for i := 1; i <= len(lines); i++ {
		if !covered[i] {
			if !inUncovered {
				uncoveredStart = i
				inUncovered = true
			}
		} else if inUncovered {
			// End of uncovered range.
			content := strings.Join(lines[uncoveredStart-1:i-1], "\n")
			if strings.TrimSpace(content) != "" && !isPreambleOnly(content, lang) {
				chunks = append(chunks, Chunk{
					ID:        fmt.Sprintf("chunk_%03d", *chunkIdx),
					FilePath:  filePath,
					StartLine: uncoveredStart,
					EndLine:   i - 1,
					Content:   content,
					Context:   preamble,
					Language:  lang,
					RiskLevel: "low",
					BestOfK:   1, // Interstitial code usually doesn't need retries.
				})
				*chunkIdx++
			}
			inUncovered = false
		}
	}

	// Handle trailing uncovered code.
	if inUncovered {
		content := strings.Join(lines[uncoveredStart-1:], "\n")
		if strings.TrimSpace(content) != "" && !isPreambleOnly(content, lang) {
			chunks = append(chunks, Chunk{
				ID:        fmt.Sprintf("chunk_%03d", *chunkIdx),
				FilePath:  filePath,
				StartLine: uncoveredStart,
				EndLine:   len(lines),
				Content:   content,
				Context:   preamble,
				Language:  lang,
				RiskLevel: "low",
				BestOfK:   1,
			})
			*chunkIdx++
		}
	}

	return chunks
}

// extractPreamble extracts the file's import/package/module header.
// This is injected as context into each chunk so scanners understand dependencies.
func extractPreamble(lines []string, boundaries []functionBoundary, lang string) string {
	if len(boundaries) == 0 {
		return ""
	}

	// Preamble is everything before the first function.
	endLine := boundaries[0].startLine - 1
	if endLine <= 0 {
		return ""
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}

	preamble := strings.Join(lines[:endLine], "\n")
	return strings.TrimSpace(preamble)
}

// isPreambleOnly checks if content is just imports/package declarations.
func isPreambleOnly(content, lang string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") ||
			strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "import ") ||
			strings.HasPrefix(trimmed, "import(") || trimmed == "import (" || trimmed == ")" ||
			strings.HasPrefix(trimmed, "from ") || strings.HasPrefix(trimmed, "require ") ||
			strings.HasPrefix(trimmed, "use ") || strings.HasPrefix(trimmed, "using ") {
			continue
		}
		return false // Non-preamble line found.
	}
	return true
}

// ── Language Detection ───────────────────────────────────────────────────────

// detectLanguage returns the programming language based on file extension.
func detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if lang, ok := extensionToLanguage[ext]; ok {
		return lang
	}
	return "unknown"
}

var extensionToLanguage = map[string]string{
	".go":    "go",
	".py":    "python",
	".js":    "javascript",
	".jsx":   "javascript",
	".ts":    "typescript",
	".tsx":   "typescript",
	".java":  "java",
	".c":     "c",
	".h":     "c",
	".cpp":   "cpp",
	".cc":    "cpp",
	".hpp":   "cpp",
	".rs":    "rust",
	".rb":    "ruby",
	".php":   "php",
	".cs":    "csharp",
	".swift": "swift",
	".kt":    "kotlin",
	".scala": "scala",
	".sol":   "solidity",
}

// ── Function Patterns ────────────────────────────────────────────────────────

// functionPatterns maps language to regex patterns for detecting function definitions.
var functionPatterns = map[string][]*regexp.Regexp{
	"go": {
		regexp.MustCompile(`^func\s+(?:\([^)]+\)\s+)?(\w+)\s*\(`),
	},
	"python": {
		regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)\s*\(`),
		regexp.MustCompile(`^class\s+(\w+)\s*[:(]`),
	},
	"javascript": {
		regexp.MustCompile(`^(?:async\s+)?function\s+(\w+)\s*\(`),
		regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`),
		regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?function`),
		regexp.MustCompile(`^class\s+(\w+)\s*(?:extends\s+\w+\s*)?{`),
	},
	"typescript": {
		regexp.MustCompile(`^(?:async\s+)?function\s+(\w+)\s*[<(]`),
		regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?(?:const|let|var)\s+(\w+)\s*[=:]\s*(?:async\s+)?\(`),
		regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?function`),
		regexp.MustCompile(`^(?:export\s+)?class\s+(\w+)`),
		regexp.MustCompile(`^(?:export\s+)?interface\s+(\w+)`),
	},
	"java": {
		regexp.MustCompile(`^\s*(?:public|private|protected|static|\s)+[\w<>\[\]]+\s+(\w+)\s*\(`),
		regexp.MustCompile(`^(?:public\s+)?class\s+(\w+)`),
	},
	"c": {
		regexp.MustCompile(`^(?:static\s+)?(?:inline\s+)?(?:const\s+)?[\w*]+\s+(\w+)\s*\(`),
	},
	"cpp": {
		regexp.MustCompile(`^(?:static\s+)?(?:inline\s+)?(?:virtual\s+)?(?:const\s+)?[\w*:&<>]+\s+(\w+)\s*\(`),
		regexp.MustCompile(`^class\s+(\w+)`),
	},
	"php": {
		regexp.MustCompile(`^\s*(?:public|private|protected|static|\s)*function\s+(\w+)\s*\(`),
		regexp.MustCompile(`^class\s+(\w+)`),
	},
	"ruby": {
		regexp.MustCompile(`^\s*def\s+(\w+)`),
		regexp.MustCompile(`^class\s+(\w+)`),
	},
	"rust": {
		regexp.MustCompile(`^(?:pub\s+)?(?:async\s+)?fn\s+(\w+)`),
		regexp.MustCompile(`^(?:pub\s+)?struct\s+(\w+)`),
		regexp.MustCompile(`^impl\s+(?:<[^>]+>\s+)?(\w+)`),
	},
}

// ── Risk Classification ─────────────────────────────────────────────────────

// classifyRisk determines the risk level of a chunk based on its function name
// and content. High-risk chunks get more best-of-K attempts.
func classifyRisk(funcName, content string) string {
	lower := strings.ToLower(funcName + " " + content)

	// High-risk indicators: auth, crypto, input parsing, network handling.
	highRiskPatterns := []string{
		"auth", "login", "password", "credential", "token", "session",
		"crypto", "encrypt", "decrypt", "hash", "sign", "verify",
		"parse", "decode", "deserialize", "unmarshal", "unpack",
		"exec", "eval", "system", "popen", "spawn",
		"query", "sql", "select ", "insert ", "update ", "delete ",
		"upload", "download", "write_file", "writefile",
		"admin", "privilege", "permission", "rbac", "acl",
		"redirect", "forward", "proxy", "cors",
		"memcpy", "strcpy", "strcat", "sprintf",
		"xdr_decode", "rpc", "socket", "recv", "recvfrom",
	}

	for _, pattern := range highRiskPatterns {
		if strings.Contains(lower, pattern) {
			return "high"
		}
	}

	// Low-risk indicators: tests, constants, documentation.
	lowRiskPatterns := []string{
		"test_", "_test", "spec_", "mock_",
		"const ", "constant", "enum",
		"string()", "tostring", "display",
	}

	for _, pattern := range lowRiskPatterns {
		if strings.Contains(lower, pattern) {
			return "low"
		}
	}

	return "medium"
}

// ── Scanner Helpers ─────────────────────────────────────────────────────────

// BuildScannerPrompt creates the Vidoc-style scanner prompt for a chunk.
// This is the prompt sent to each chunk-scanner subagent.
func (ch *Chunk) BuildScannerPrompt(knowledgeInjection string, totalChunks int) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Task: Scan `%s` for concrete, evidence-backed vulnerabilities.\n", ch.FilePath))
	b.WriteString("Report only real issues in the target file.\n\n")

	if ch.FunctionName != "" {
		b.WriteString(fmt.Sprintf("Assigned chunk %s of %d: `%s`.\n", ch.ID, totalChunks, ch.FunctionName))
	} else {
		b.WriteString(fmt.Sprintf("Assigned chunk %s of %d.\n", ch.ID, totalChunks))
	}
	b.WriteString(fmt.Sprintf("Focus on lines %d-%d.\n", ch.StartLine, ch.EndLine))
	b.WriteString("You may inspect any repository file to confirm or refute behavior.\n\n")

	if ch.Context != "" {
		b.WriteString("FILE CONTEXT (imports, types, surrounding signatures):\n")
		b.WriteString("```\n")
		b.WriteString(ch.Context)
		b.WriteString("\n```\n\n")
	}

	b.WriteString("CODE TO ANALYZE:\n")
	b.WriteString("```" + ch.Language + "\n")
	b.WriteString(ch.Content)
	b.WriteString("\n```\n\n")

	if knowledgeInjection != "" {
		b.WriteString("KNOWN VULNERABILITY PATTERNS FOR THIS CONTEXT:\n")
		b.WriteString(knowledgeInjection)
		b.WriteString("\n\n")
	}

	b.WriteString(`For each vulnerability found, report EXACTLY this JSON structure:
{
  "findings": [
    {
      "title": "Short descriptive title",
      "description": "Detailed explanation with evidence",
      "severity": "critical|high|medium|low|info",
      "cwe": "CWE-NNN",
      "confidence": "high|medium|low",
      "start_line": <line number>,
      "end_line": <line number>,
      "vulnerable_code": "the specific vulnerable code snippet",
      "remediation": "how to fix it"
    }
  ]
}

If no vulnerabilities are found, return: {"findings": []}
`)

	return b.String()
}
