// Package scanners provides a unified interface for external static analysis
// tools (semgrep, CodeQL, trivy, etc.) and normalizes their output into the
// Aegis pipeline's RawFinding format.
//
// The package supports SARIF (Static Analysis Results Interchange Format) as
// the primary interchange format — most modern tools can output SARIF:
//
//   - semgrep:  semgrep --sarif --output results.sarif
//   - CodeQL:   codeql database analyze --format=sarif-latest
//   - ESLint:   eslint --format @microsoft/eslint-formatter-sarif
//   - Bandit:   bandit -f sarif
//   - Checkov:  checkov --output sarif
//
// Usage:
//
//	registry := scanners.NewRegistry()
//	results, err := registry.Run(ctx, "semgrep", "/path/to/workspace")
//	for _, f := range results {
//	    fmt.Printf("[%s] %s — %s:%d\n", f.Severity, f.Title, f.FilePath, f.StartLine)
//	}
package scanners

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"

	"github.com/pixelvide/aegis/agent/pipeline"
)

// ── Scanner Interface ────────────────────────────────────────────────────────

// Scanner defines the contract for an external static analysis tool adapter.
type Scanner interface {
	// Name returns the scanner's identifier (e.g., "semgrep", "codeql").
	Name() string

	// Available checks if the scanner's binary is installed and accessible.
	Available() bool

	// Scan runs the scanner on the given workspace directory and returns
	// normalized findings. The workspace path is validated before being passed.
	Scan(ctx context.Context, workspace string) ([]pipeline.RawFinding, error)
}

// ── Registry ─────────────────────────────────────────────────────────────────

// Registry manages available scanners and provides a unified interface
// for discovering, running, and aggregating scanner results.
type Registry struct {
	mu       sync.RWMutex
	scanners map[string]Scanner
}

// NewRegistry creates a new scanner registry pre-populated with all
// built-in scanner adapters.
func NewRegistry() *Registry {
	r := &Registry{
		scanners: make(map[string]Scanner),
	}

	// Register built-in adapters.
	r.Register(&SemgrepScanner{})
	r.Register(&OpenGrepScanner{})
	r.Register(&TrivyScanner{})
	r.Register(&BanditScanner{})
	r.Register(&GosecScanner{})

	return r
}

// Register adds a scanner to the registry.
func (r *Registry) Register(s Scanner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scanners[s.Name()] = s
}

// Get returns a scanner by name.
func (r *Registry) Get(name string) (Scanner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.scanners[name]
	return s, ok
}

// Available returns all scanners whose binaries are installed.
func (r *Registry) Available() []Scanner {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var avail []Scanner
	for _, s := range r.scanners {
		if s.Available() {
			avail = append(avail, s)
		}
	}
	return avail
}

// All returns all registered scanners regardless of availability.
func (r *Registry) All() []Scanner {
	r.mu.RLock()
	defer r.mu.RUnlock()

	all := make([]Scanner, 0, len(r.scanners))
	for _, s := range r.scanners {
		all = append(all, s)
	}
	return all
}

// Run invokes a specific scanner by name and returns normalized findings.
func (r *Registry) Run(ctx context.Context, name, workspace string) ([]pipeline.RawFinding, error) {
	s, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("scanner %q not registered", name)
	}

	if !s.Available() {
		return nil, fmt.Errorf("scanner %q is not installed (binary not found in PATH)", name)
	}

	slog.Info("scanner: running", "scanner", name, "workspace", workspace)

	findings, err := s.Scan(ctx, workspace)
	if err != nil {
		return nil, fmt.Errorf("scanner %q failed: %w", name, err)
	}

	slog.Info("scanner: complete", "scanner", name, "findings", len(findings))

	// Tag all findings with the scanner source.
	for i := range findings {
		findings[i].Source = name
	}

	return findings, nil
}

// RunAll runs every available scanner and aggregates results.
func (r *Registry) RunAll(ctx context.Context, workspace string) ([]pipeline.RawFinding, error) {
	avail := r.Available()
	if len(avail) == 0 {
		slog.Info("scanner: no external scanners available")
		return nil, nil
	}

	slog.Info("scanner: running all available", "count", len(avail))

	var allFindings []pipeline.RawFinding
	for _, s := range avail {
		findings, err := r.Run(ctx, s.Name(), workspace)
		if err != nil {
			slog.Warn("scanner: tool failed, continuing", "scanner", s.Name(), "error", err)
			continue
		}
		allFindings = append(allFindings, findings...)
	}

	return allFindings, nil
}

// ListInfo returns a human-readable summary of all registered scanners
// and their availability status.
func (r *Registry) ListInfo() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b strings.Builder
	b.WriteString("External Scanners:\n")
	for _, s := range r.scanners {
		status := "❌ not installed"
		if s.Available() {
			status = "✅ available"
		}
		b.WriteString(fmt.Sprintf("  %-15s %s\n", s.Name(), status))
	}
	return b.String()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// binaryExists checks if a binary is available in PATH.
// This is the standard way to check scanner availability.
func binaryExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
