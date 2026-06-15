// Package models defines the domain types for the Aegis security platform.
//
// These types are used across the server: API handlers serialize them to JSON,
// the store layer persists them, and the orchestrator creates them from agent output.
package models

import "time"

// ScanStatus represents the lifecycle state of a scan.
type ScanStatus string

const (
	ScanPending   ScanStatus = "pending"
	ScanRunning   ScanStatus = "running"
	ScanCompleted ScanStatus = "completed"
	ScanFailed    ScanStatus = "failed"
	ScanCancelled ScanStatus = "cancelled"
)

// ExecMode controls how the agent process is launched.
type ExecMode string

const (
	ExecDirect ExecMode = "direct" // Subprocess on same host
	ExecDocker ExecMode = "docker" // Docker container (sandboxed)
)

// Scan represents a security analysis run.
type Scan struct {
	ID             string     `json:"id"`
	ProjectID      string     `json:"project_id,omitempty"`
	Name           string     `json:"name"`
	Target         Target     `json:"target"`
	Persona        string     `json:"persona"` // sharingan, senku, killua
	Mode           ExecMode   `json:"mode"`
	Status         ScanStatus `json:"status"`
	Prompt         string     `json:"prompt,omitempty"`
	AgentPID       int        `json:"agent_pid,omitempty"`
	ConversationID string     `json:"conversation_id,omitempty"`
	WorkspacePath  string     `json:"workspace_path,omitempty"`
	FindingCount   int        `json:"finding_count"`
	Summary        *Summary   `json:"summary,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// Target describes what the agent should analyze.
type Target struct {
	Type string `json:"type"` // "path", "git", "url"
	Path string `json:"path,omitempty"`
	URL  string `json:"url,omitempty"`
	Ref  string `json:"ref,omitempty"` // git branch/tag
}

// Summary is a severity breakdown of findings for a scan.
type Summary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}
