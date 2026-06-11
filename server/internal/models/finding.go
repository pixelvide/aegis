package models

import "time"

// FindingStatus represents the triage state of a finding.
type FindingStatus string

const (
	FindingOpen          FindingStatus = "open"
	FindingConfirmed     FindingStatus = "confirmed"
	FindingFixed         FindingStatus = "fixed"
	FindingFalsePositive FindingStatus = "false_positive"
	FindingWontFix       FindingStatus = "wontfix"
	FindingVerified      FindingStatus = "verified" // agent confirmed fix is effective
)

// Severity levels ordered by criticality.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// Finding represents a vulnerability discovered by an agent.
type Finding struct {
	ID          string        `json:"id"`
	ScanID      string        `json:"scan_id"`
	ProjectID   string        `json:"project_id,omitempty"`
	Fingerprint string        `json:"fingerprint,omitempty"` // stable dedup key (agent-computed)
	Title       string        `json:"title"`
	Severity    Severity      `json:"severity"`
	CWE         string        `json:"cwe,omitempty"`
	OWASP       string        `json:"owasp,omitempty"`
	CVE         string        `json:"cve,omitempty"`
	CVSSScore   float64       `json:"cvss_score,omitempty"`
	CVSSVector  string        `json:"cvss_vector,omitempty"`
	File        string        `json:"file,omitempty"`
	Line        int           `json:"line,omitempty"`
	Status      FindingStatus `json:"status"`
	Description string        `json:"description"`
	Source      string        `json:"source,omitempty"`
	SeenCount   int           `json:"seen_count"`
	LastSeenAt  time.Time     `json:"last_seen_at"`
	Exploits    []Exploit     `json:"exploits,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Exploit is a proof-of-concept script attached to a finding.
type Exploit struct {
	ID        string `json:"id"`
	FindingID string `json:"finding_id"`
	Filename  string `json:"filename"` // exploit.sh, exploit.py
	Language  string `json:"language"` // bash, python, javascript
	Code      string `json:"code"`     // Source code of the exploit
	Validated bool   `json:"validated"`
}
