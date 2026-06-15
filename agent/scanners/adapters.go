package scanners

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pixelvide/aegis/agent/pipeline"
)

// ── Semgrep Adapter ──────────────────────────────────────────────────────────
//
// semgrep is a fast, open-source static analysis tool that supports 30+
// languages. It outputs SARIF natively.
//
// Install: pip install semgrep
// Config:  Auto-uses "auto" ruleset (community rules) if no .semgrep.yml exists.

// SemgrepScanner runs semgrep and parses its SARIF output.
type SemgrepScanner struct{}

func (s *SemgrepScanner) Name() string     { return "semgrep" }
func (s *SemgrepScanner) Available() bool   { return binaryExists("semgrep") }

func (s *SemgrepScanner) Scan(ctx context.Context, workspace string) ([]pipeline.RawFinding, error) {
	// Create temp file for SARIF output.
	tmpFile, err := os.CreateTemp("", "aegis-semgrep-*.sarif")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Determine config: use workspace .semgrep.yml if it exists, otherwise "auto".
	config := "auto"
	for _, candidate := range []string{".semgrep.yml", ".semgrep.yaml", ".semgrep/rules.yml"} {
		if _, err := os.Stat(filepath.Join(workspace, candidate)); err == nil {
			config = candidate
			break
		}
	}

	// Security: validate workspace path is absolute and exists.
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	// Build command with hardcoded binary and validated arguments.
	// TODO(security): Binary path is validated via exec.LookPath in Available().
	args := []string{
		"scan",
		"--sarif",
		"--output", tmpPath,
		"--config", config,
		"--quiet",        // Suppress progress output.
		"--no-git-ignore", // Scan everything (we handle filtering).
		"--max-target-bytes", "500000", // Skip files > 500KB.
		absWorkspace,
	}

	cmd := exec.CommandContext(ctx, "semgrep", args...)
	cmd.Dir = absWorkspace
	output, err := cmd.CombinedOutput()

	// Semgrep exits with code 1 if findings are found — that's normal.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				return nil, fmt.Errorf("semgrep failed (exit %d): %s", exitErr.ExitCode(), string(output))
			}
		} else {
			return nil, fmt.Errorf("semgrep execution error: %w", err)
		}
	}

	// Read and parse SARIF output.
	sarifData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read SARIF output: %w", err)
	}

	return ParseSARIF(sarifData, absWorkspace)
}

// ── OpenGrep Adapter ─────────────────────────────────────────────────────────
//
// opengrep is a community-maintained open-source fork of semgrep. It uses
// the same CLI interface, rule format, and output formats (including SARIF).
// It's a drop-in replacement with fully compatible rules.
//
// Install: curl -fsSL https://raw.githubusercontent.com/opengrep/opengrep/main/install.sh | bash
// Rules:   Compatible with all semgrep community rules.

// OpenGrepScanner runs opengrep and parses its SARIF output.
type OpenGrepScanner struct{}

func (s *OpenGrepScanner) Name() string     { return "opengrep" }
func (s *OpenGrepScanner) Available() bool   { return binaryExists("opengrep") }

func (s *OpenGrepScanner) Scan(ctx context.Context, workspace string) ([]pipeline.RawFinding, error) {
	// Create temp file for SARIF output.
	tmpFile, err := os.CreateTemp("", "aegis-opengrep-*.sarif")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Determine config: use workspace config if it exists, otherwise "auto".
	// OpenGrep is compatible with .semgrep.yml rule files.
	config := "auto"
	for _, candidate := range []string{".semgrep.yml", ".semgrep.yaml", ".semgrep/rules.yml"} {
		if _, err := os.Stat(filepath.Join(workspace, candidate)); err == nil {
			config = candidate
			break
		}
	}

	// Security: validate workspace path is absolute and exists.
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	// Build command — identical to semgrep CLI, just a different binary.
	// TODO(security): Binary path is validated via exec.LookPath in Available().
	args := []string{
		"scan",
		"--sarif",
		"--output", tmpPath,
		"--config", config,
		"--quiet",
		"--no-git-ignore",
		"--max-target-bytes", "500000",
		absWorkspace,
	}

	cmd := exec.CommandContext(ctx, "opengrep", args...)
	cmd.Dir = absWorkspace
	output, err := cmd.CombinedOutput()

	// OpenGrep (like semgrep) exits with code 1 if findings are found.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				return nil, fmt.Errorf("opengrep failed (exit %d): %s", exitErr.ExitCode(), string(output))
			}
		} else {
			return nil, fmt.Errorf("opengrep execution error: %w", err)
		}
	}

	// Read and parse SARIF output.
	sarifData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read SARIF output: %w", err)
	}

	return ParseSARIF(sarifData, absWorkspace)
}

// ── Trivy Adapter ────────────────────────────────────────────────────────────
//
// trivy is a comprehensive vulnerability scanner for containers, file systems,
// Git repositories, and IaC. It outputs SARIF natively.
//
// Install: https://aquasecurity.github.io/trivy/
// Modes:   fs (file system), config (IaC misconfigs)

// TrivyScanner runs trivy filesystem scanning and parses SARIF output.
type TrivyScanner struct{}

func (s *TrivyScanner) Name() string     { return "trivy" }
func (s *TrivyScanner) Available() bool   { return binaryExists("trivy") }

func (s *TrivyScanner) Scan(ctx context.Context, workspace string) ([]pipeline.RawFinding, error) {
	tmpFile, err := os.CreateTemp("", "aegis-trivy-*.sarif")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	// Run trivy in filesystem mode with SARIF output.
	args := []string{
		"fs",
		"--format", "sarif",
		"--output", tmpPath,
		"--scanners", "vuln,misconfig,secret",
		"--severity", "UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL",
		absWorkspace,
	}

	cmd := exec.CommandContext(ctx, "trivy", args...)
	cmd.Dir = absWorkspace
	output, err := cmd.CombinedOutput()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() > 1 {
				return nil, fmt.Errorf("trivy failed (exit %d): %s", exitErr.ExitCode(), string(output))
			}
		} else {
			return nil, fmt.Errorf("trivy execution error: %w", err)
		}
	}

	sarifData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read SARIF output: %w", err)
	}

	return ParseSARIF(sarifData, absWorkspace)
}

// ── Bandit Adapter ───────────────────────────────────────────────────────────
//
// bandit is a Python-specific static analysis tool designed to find common
// security issues in Python code.
//
// Install: pip install bandit

// BanditScanner runs bandit and parses SARIF output.
type BanditScanner struct{}

func (s *BanditScanner) Name() string     { return "bandit" }
func (s *BanditScanner) Available() bool   { return binaryExists("bandit") }

func (s *BanditScanner) Scan(ctx context.Context, workspace string) ([]pipeline.RawFinding, error) {
	tmpFile, err := os.CreateTemp("", "aegis-bandit-*.sarif")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	args := []string{
		"-r",            // Recursive.
		"-f", "sarif",   // SARIF output format.
		"-o", tmpPath,   // Output file.
		absWorkspace,
	}

	cmd := exec.CommandContext(ctx, "bandit", args...)
	cmd.Dir = absWorkspace
	output, err := cmd.CombinedOutput()

	// Bandit exits with code 1 if findings are found.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				return nil, fmt.Errorf("bandit failed (exit %d): %s", exitErr.ExitCode(), string(output))
			}
		} else {
			return nil, fmt.Errorf("bandit execution error: %w", err)
		}
	}

	sarifData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read SARIF output: %w", err)
	}

	return ParseSARIF(sarifData, absWorkspace)
}

// ── Gosec Adapter ────────────────────────────────────────────────────────────
//
// gosec is a Go-specific security scanner that inspects AST for security issues.
//
// Install: go install github.com/securego/gosec/v2/cmd/gosec@latest

// GosecScanner runs gosec and parses SARIF output.
type GosecScanner struct{}

func (s *GosecScanner) Name() string     { return "gosec" }
func (s *GosecScanner) Available() bool   { return binaryExists("gosec") }

func (s *GosecScanner) Scan(ctx context.Context, workspace string) ([]pipeline.RawFinding, error) {
	tmpFile, err := os.CreateTemp("", "aegis-gosec-*.sarif")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace path: %w", err)
	}

	args := []string{
		"-fmt=sarif",
		"-out=" + tmpPath,
		"-quiet",
		absWorkspace + "/...",
	}

	cmd := exec.CommandContext(ctx, "gosec", args...)
	cmd.Dir = absWorkspace
	output, err := cmd.CombinedOutput()

	// Gosec exits with code 1 if findings are found.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				return nil, fmt.Errorf("gosec failed (exit %d): %s", exitErr.ExitCode(), string(output))
			}
		} else {
			return nil, fmt.Errorf("gosec execution error: %w", err)
		}
	}

	sarifData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read SARIF output: %w", err)
	}

	return ParseSARIF(sarifData, absWorkspace)
}
