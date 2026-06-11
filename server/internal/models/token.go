package models

import "time"

// APIToken represents an API token for agent authentication.
// Tokens are stored per-org (in the org schema, not common).
// The plaintext token is shown once at creation and never stored.
type APIToken struct {
	ID        string     `json:"id"`
	ProjectID string     `json:"project_id,omitempty"` // empty = org-wide
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`                // "aegis_a1b2c3d4" for display
	CreatedBy string     `json:"created_by"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Revoked   bool       `json:"revoked"`
	CreatedAt time.Time  `json:"created_at"`
}
