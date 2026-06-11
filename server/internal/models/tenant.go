package models

import "time"

// Organization represents a tenant in the multi-tenant system.
// Each org gets its own PostgreSQL schema: org_{ID without hyphens}.
type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"` // user-facing URL identifier, changeable
	Plan      string    `json:"plan"` // free, pro, enterprise
	CreatedAt time.Time `json:"created_at"`
}

// SchemaName returns the PostgreSQL schema name for this org.
// Uses the full UUID (hyphens stripped) to guarantee uniqueness.
func (o *Organization) SchemaName() string {
	return OrgSchemaName(o.ID)
}

// OrgSchemaName converts an org ID to its PostgreSQL schema name.
func OrgSchemaName(orgID string) string {
	// Strip hyphens from UUID: a1b2c3d4-e5f6-... → a1b2c3d4e5f6...
	clean := make([]byte, 0, 32)
	for i := range orgID {
		if orgID[i] != '-' {
			clean = append(clean, orgID[i])
		}
	}
	return "org_" + string(clean)
}

// User represents an authenticated user.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	AvatarURL    string    `json:"avatar_url"`
	PasswordHash string    `json:"-"` // never serialized to client
	CreatedAt    time.Time `json:"created_at"`
}

// OrgMember represents a user's membership in an organization.
type OrgMember struct {
	OrgID  string `json:"org_id"`
	UserID string `json:"user_id"`
	Role   string `json:"role"` // owner, admin, member, viewer
}

// Project represents a project within an organization.
// Stored in the org's schema.
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
}
