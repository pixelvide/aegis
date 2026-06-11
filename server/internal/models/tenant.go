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
	ID            string    `json:"id"`
	Email         string    `json:"email"`          // primary email (denormalized from user_emails)
	Name          string    `json:"name"`
	AvatarURL     string    `json:"avatar_url"`
	PasswordHash  string    `json:"-"`              // never serialized to client
	MFAEnabled    bool      `json:"mfa_enabled"`    // manual toggle — not auto-computed
	EmailVerified bool      `json:"email_verified"` // computed: is primary email verified? (not a DB column)
	CreatedAt     time.Time `json:"created_at"`
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

// ─── User Emails ────────────────────────────────────────────────────────────

// UserEmail represents one of a user's email addresses.
type UserEmail struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Email      string     `json:"email"`
	IsPrimary  bool       `json:"is_primary"`
	Verified   bool       `json:"verified"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ─── MFA Devices ────────────────────────────────────────────────────────────

// MFADevice represents a registered MFA method (TOTP or email OTP).
type MFADevice struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`            // "totp" | "email"
	Secret     string     `json:"-"`               // TOTP secret — never exposed
	Email      string     `json:"email,omitempty"` // target email (for email type only)
	Verified   bool       `json:"verified"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ─── User Sessions ──────────────────────────────────────────────────────────

// UserSession represents an active login session.
type UserSession struct {
	ID           string     `json:"id"`
	UserID       string     `json:"-"`
	JTI          string     `json:"-"`              // JWT ID — never exposed
	IPAddress    string     `json:"ip_address"`
	UserAgent    string     `json:"user_agent"`
	Browser      string     `json:"browser"`
	OS           string     `json:"os"`
	DeviceType   string     `json:"device_type"`    // desktop, mobile, tablet
	CreatedAt    time.Time  `json:"created_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	RevokedAt    *time.Time `json:"-"`
	Current      bool       `json:"current"`        // computed: is this the caller's session?
}
