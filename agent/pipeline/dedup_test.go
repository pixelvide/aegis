package pipeline

import (
	"testing"
)

func TestDedup_ExactMatch(t *testing.T) {
	d := NewDeduplicator()

	raw := []RawFinding{
		{ChunkID: "c1", AttemptIndex: 0, FilePath: "auth.go", StartLine: 10, EndLine: 15, Title: "SQLi in Login", Severity: "critical", CWE: "CWE-89", Confidence: "high"},
		{ChunkID: "c1", AttemptIndex: 1, FilePath: "auth.go", StartLine: 10, EndLine: 15, Title: "SQL Injection in Login", Severity: "critical", CWE: "CWE-89", Confidence: "high"},
		{ChunkID: "c1", AttemptIndex: 2, FilePath: "auth.go", StartLine: 10, EndLine: 15, Title: "SQLi", Severity: "high", CWE: "CWE-89", Confidence: "medium"},
	}

	result := d.Dedup(raw)
	if len(result) != 1 {
		t.Fatalf("expected 1 deduped finding, got %d", len(result))
	}

	// Should pick the highest severity/confidence.
	if result[0].Severity != "critical" {
		t.Errorf("expected severity 'critical', got %q", result[0].Severity)
	}
	if result[0].SourceCount != 3 {
		t.Errorf("expected source count 3, got %d", result[0].SourceCount)
	}
}

func TestDedup_NearMatch(t *testing.T) {
	d := NewDeduplicator()

	raw := []RawFinding{
		{ChunkID: "c1", AttemptIndex: 0, FilePath: "auth.go", StartLine: 10, EndLine: 15, Title: "XSS on line 10", Severity: "high", CWE: "CWE-79", Confidence: "high"},
		{ChunkID: "c1", AttemptIndex: 1, FilePath: "auth.go", StartLine: 12, EndLine: 17, Title: "XSS on line 12", Severity: "high", CWE: "CWE-79", Confidence: "medium"},
	}

	result := d.Dedup(raw)
	// Lines 10 and 12 with same CWE in same file should merge (within 5-line threshold).
	if len(result) != 1 {
		t.Fatalf("expected 1 deduped finding (near match), got %d", len(result))
	}

	// Merged range should span both findings.
	if result[0].StartLine != 10 || result[0].EndLine != 17 {
		t.Errorf("expected merged range 10-17, got %d-%d", result[0].StartLine, result[0].EndLine)
	}
}

func TestDedup_DifferentVulns(t *testing.T) {
	d := NewDeduplicator()

	raw := []RawFinding{
		{ChunkID: "c1", AttemptIndex: 0, FilePath: "auth.go", StartLine: 10, Title: "SQLi", Severity: "critical", CWE: "CWE-89", Confidence: "high"},
		{ChunkID: "c1", AttemptIndex: 0, FilePath: "auth.go", StartLine: 50, Title: "XSS", Severity: "high", CWE: "CWE-79", Confidence: "high"},
		{ChunkID: "c2", AttemptIndex: 0, FilePath: "api.go", StartLine: 100, Title: "Path Traversal", Severity: "high", CWE: "CWE-22", Confidence: "medium"},
	}

	result := d.Dedup(raw)
	if len(result) != 3 {
		t.Fatalf("expected 3 different findings to remain, got %d", len(result))
	}
}

func TestDedup_Empty(t *testing.T) {
	d := NewDeduplicator()

	result := d.Dedup(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestDedup_SortOrder(t *testing.T) {
	d := NewDeduplicator()

	raw := []RawFinding{
		{ChunkID: "c1", AttemptIndex: 0, FilePath: "z.go", StartLine: 10, Title: "A", Severity: "high", CWE: "CWE-1"},
		{ChunkID: "c2", AttemptIndex: 0, FilePath: "a.go", StartLine: 50, Title: "B", Severity: "high", CWE: "CWE-2"},
		{ChunkID: "c3", AttemptIndex: 0, FilePath: "a.go", StartLine: 10, Title: "C", Severity: "high", CWE: "CWE-3"},
	}

	result := d.Dedup(raw)
	if len(result) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(result))
	}

	// Should be sorted by file path, then start line.
	if result[0].FilePath != "a.go" || result[0].StartLine != 10 {
		t.Errorf("expected first finding to be a.go:10, got %s:%d", result[0].FilePath, result[0].StartLine)
	}
	if result[1].FilePath != "a.go" || result[1].StartLine != 50 {
		t.Errorf("expected second finding to be a.go:50, got %s:%d", result[1].FilePath, result[1].StartLine)
	}
	if result[2].FilePath != "z.go" {
		t.Errorf("expected third finding to be z.go, got %s", result[2].FilePath)
	}
}

func TestNormalizeCWE(t *testing.T) {
	tests := map[string]string{
		"CWE-89":  "CWE-89",
		"cwe-89":  "CWE-89",
		"CWE 89":  "CWE-89",
		"89":      "CWE-89",
		"":        "",
		"  CWE-79 ": "CWE-79",
	}

	for input, expected := range tests {
		got := normalizeCWE(input)
		if got != expected {
			t.Errorf("normalizeCWE(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestSeverityRank(t *testing.T) {
	if severityRank("critical") <= severityRank("high") {
		t.Error("critical should rank higher than high")
	}
	if severityRank("high") <= severityRank("medium") {
		t.Error("high should rank higher than medium")
	}
	if severityRank("medium") <= severityRank("low") {
		t.Error("medium should rank higher than low")
	}
}
