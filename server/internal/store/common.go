package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pixelvide/aegis/server/internal/models"
)

// CommonStore manages the shared schema: users, organizations, memberships.
type CommonStore struct {
	db *sql.DB
}

// NewCommonStore initializes the common schema and returns a CommonStore.
func NewCommonStore(db *sql.DB) (*CommonStore, error) {
	cs := &CommonStore{db: db}
	if err := cs.migrate(); err != nil {
		return nil, fmt.Errorf("common schema migrate: %w", err)
	}
	return cs, nil
}

// migration represents a single versioned schema change.
type migration struct {
	Version     int
	Description string
	SQL         string
}

// migrations is the ordered list of all schema migrations.
// APPEND ONLY — never edit or reorder existing entries.
var migrations = []migration{
	{
		Version:     1,
		Description: "Initial schema: organizations, users, org_members",
		SQL: `
		CREATE SCHEMA IF NOT EXISTS common;

		CREATE TABLE IF NOT EXISTS common.organizations (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name       TEXT NOT NULL,
			slug       TEXT NOT NULL UNIQUE,
			plan       TEXT NOT NULL DEFAULT 'free',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS common.users (
			id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email         TEXT NOT NULL UNIQUE,
			name          TEXT NOT NULL DEFAULT '',
			avatar_url    TEXT NOT NULL DEFAULT '',
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS common.org_members (
			org_id  UUID NOT NULL REFERENCES common.organizations(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			role    TEXT NOT NULL DEFAULT 'member',
			PRIMARY KEY (org_id, user_id)
		);
		`,
	},
	{
		Version:     2,
		Description: "Add password_hash to users",
		SQL:         `ALTER TABLE common.users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';`,
	},
	{
		Version:     3,
		Description: "App config and feature flags tables",
		SQL: `
		CREATE TABLE IF NOT EXISTS common.app_config (
			key        TEXT PRIMARY KEY,
			value      TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS common.feature_flags (
			name        TEXT PRIMARY KEY,
			enabled     BOOLEAN NOT NULL DEFAULT FALSE,
			description TEXT NOT NULL DEFAULT '',
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		`,
	},
	{
		Version:     4,
		Description: "Seed default feature flags",
		SQL: `
		INSERT INTO common.feature_flags (name, enabled, description) VALUES
			('signup',            TRUE,  'Allow new user registration'),
			('invite_only',       FALSE, 'Only allow signups via org invite'),
			('scan_docker_mode',  FALSE, 'Enable Docker sandbox mode for scans'),
			('public_api',        FALSE, 'Allow unauthenticated API access')
		ON CONFLICT (name) DO NOTHING;
		`,
	},
	{
		Version:     5,
		Description: "Add custom_domain to organizations",
		SQL: `
		ALTER TABLE common.organizations ADD COLUMN IF NOT EXISTS custom_domain TEXT DEFAULT '';
		CREATE UNIQUE INDEX IF NOT EXISTS idx_orgs_custom_domain
			ON common.organizations(custom_domain) WHERE custom_domain != '';
		`,
	},
	{
		Version:     6,
		Description: "Add api_tokens table and new finding columns to all org schemas",
		SQL: `
		DO $$
		DECLARE s TEXT;
		BEGIN
		  FOR s IN SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'org_%'
		  LOOP
		    -- api_tokens table
		    EXECUTE format('
		      CREATE TABLE IF NOT EXISTS %I.api_tokens (
		        id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		        project_id  UUID,
		        name        TEXT NOT NULL DEFAULT '''',
		        token_hash  TEXT NOT NULL,
		        prefix      TEXT NOT NULL DEFAULT '''',
		        created_by  UUID,
		        last_used   TIMESTAMPTZ,
		        expires_at  TIMESTAMPTZ,
		        revoked     BOOLEAN NOT NULL DEFAULT FALSE,
		        created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		      )', s);
		    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%s_tokens_prefix ON %I.api_tokens(prefix)', replace(s, 'org_', ''), s);

		    -- New finding columns
		    EXECUTE format('ALTER TABLE %I.findings ADD COLUMN IF NOT EXISTS fingerprint TEXT DEFAULT ''''', s);
		    EXECUTE format('ALTER TABLE %I.findings ADD COLUMN IF NOT EXISTS project_id UUID', s);
		    EXECUTE format('ALTER TABLE %I.findings ADD COLUMN IF NOT EXISTS cve TEXT DEFAULT ''''', s);
		    EXECUTE format('ALTER TABLE %I.findings ADD COLUMN IF NOT EXISTS cvss_score REAL DEFAULT 0', s);
		    EXECUTE format('ALTER TABLE %I.findings ADD COLUMN IF NOT EXISTS cvss_vector TEXT DEFAULT ''''', s);
		    EXECUTE format('ALTER TABLE %I.findings ADD COLUMN IF NOT EXISTS source TEXT DEFAULT ''''', s);
		    EXECUTE format('ALTER TABLE %I.findings ADD COLUMN IF NOT EXISTS seen_count INTEGER DEFAULT 1', s);
		    EXECUTE format('ALTER TABLE %I.findings ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ DEFAULT NOW()', s);
		    EXECUTE format('CREATE UNIQUE INDEX IF NOT EXISTS idx_%s_findings_fp ON %I.findings(fingerprint) WHERE fingerprint != ''''', replace(s, 'org_', ''), s);
		  END LOOP;
		END $$;
		`,
	},
	{
		Version:     7,
		Description: "Password reset tokens, MFA user columns, and recovery codes",
		SQL: `
		-- Password reset tokens
		CREATE TABLE IF NOT EXISTS common.password_reset_tokens (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id     UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			token_hash  TEXT NOT NULL,
			expires_at  TIMESTAMPTZ NOT NULL,
			used_at     TIMESTAMPTZ,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_reset_tokens_user ON common.password_reset_tokens(user_id);

		-- MFA fields on users
		ALTER TABLE common.users ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE common.users ADD COLUMN IF NOT EXISTS mfa_secret TEXT NOT NULL DEFAULT '';
		ALTER TABLE common.users ADD COLUMN IF NOT EXISTS mfa_verified_at TIMESTAMPTZ;

		-- MFA recovery codes (hashed)
		CREATE TABLE IF NOT EXISTS common.mfa_recovery_codes (
			id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id   UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			code_hash TEXT NOT NULL,
			used_at   TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_mfa_recovery_user ON common.mfa_recovery_codes(user_id);
		`,
	},
	{
		Version:     8,
		Description: "Email verification",
		SQL: `
		ALTER TABLE common.users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE common.users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;

		CREATE TABLE IF NOT EXISTS common.email_verification_tokens (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id    UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at    TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_email_verify_user ON common.email_verification_tokens(user_id);
		`,
	},
	{
		Version:     9,
		Description: "Multi-email, multi-device MFA, email OTP, user sessions",
		SQL: `
		-- Drop unreleased MFA/email columns from users
		ALTER TABLE common.users DROP COLUMN IF EXISTS mfa_secret;
		ALTER TABLE common.users DROP COLUMN IF EXISTS mfa_verified_at;
		ALTER TABLE common.users DROP COLUMN IF EXISTS email_verified;
		ALTER TABLE common.users DROP COLUMN IF EXISTS email_verified_at;

		-- Drop unreleased tables
		DROP TABLE IF EXISTS common.mfa_recovery_codes;
		DROP TABLE IF EXISTS common.email_verification_tokens;

		-- 1. Multiple emails per user
		CREATE TABLE common.user_emails (
			id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id     UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			email       TEXT NOT NULL UNIQUE,
			is_primary  BOOLEAN NOT NULL DEFAULT FALSE,
			verified    BOOLEAN NOT NULL DEFAULT FALSE,
			verified_at TIMESTAMPTZ,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX idx_user_emails_user ON common.user_emails(user_id);

		-- Seed existing primary emails
		INSERT INTO common.user_emails (user_id, email, is_primary, verified)
		SELECT id, email, TRUE, FALSE FROM common.users
		ON CONFLICT (email) DO NOTHING;

		-- 2. MFA devices (TOTP + email OTP)
		CREATE TABLE common.mfa_devices (
			id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id      UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			name         TEXT NOT NULL DEFAULT '',
			type         TEXT NOT NULL,
			secret       TEXT NOT NULL DEFAULT '',
			email        TEXT NOT NULL DEFAULT '',
			verified     BOOLEAN NOT NULL DEFAULT FALSE,
			verified_at  TIMESTAMPTZ,
			last_used_at TIMESTAMPTZ,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX idx_mfa_devices_user ON common.mfa_devices(user_id);

		-- 3. MFA recovery codes (8 one-time codes, bcrypt hashed)
		CREATE TABLE common.mfa_recovery_codes (
			id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id   UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			code_hash TEXT NOT NULL,
			used_at   TIMESTAMPTZ
		);
		CREATE INDEX idx_mfa_recovery_user ON common.mfa_recovery_codes(user_id);

		-- 4. Email OTP codes (for email-based MFA)
		CREATE TABLE common.email_otp_codes (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id    UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			device_id  UUID NOT NULL REFERENCES common.mfa_devices(id) ON DELETE CASCADE,
			code_hash  TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			used_at    TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX idx_email_otp_user ON common.email_otp_codes(user_id);

		-- 5. User sessions
		CREATE TABLE common.user_sessions (
			id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id        UUID NOT NULL REFERENCES common.users(id) ON DELETE CASCADE,
			jti            TEXT NOT NULL UNIQUE,
			ip_address     TEXT NOT NULL DEFAULT '',
			user_agent     TEXT NOT NULL DEFAULT '',
			browser        TEXT NOT NULL DEFAULT '',
			os             TEXT NOT NULL DEFAULT '',
			device_type    TEXT NOT NULL DEFAULT 'desktop',
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at     TIMESTAMPTZ NOT NULL,
			revoked_at     TIMESTAMPTZ
		);
		CREATE INDEX idx_sessions_user ON common.user_sessions(user_id);
		CREATE INDEX idx_sessions_jti ON common.user_sessions(jti);
		`,
	},
	{
		Version:     10,
		Description: "Add browser, os, device_type columns to user_sessions",
		SQL: `
		ALTER TABLE common.user_sessions ADD COLUMN IF NOT EXISTS browser TEXT NOT NULL DEFAULT '';
		ALTER TABLE common.user_sessions ADD COLUMN IF NOT EXISTS os TEXT NOT NULL DEFAULT '';
		ALTER TABLE common.user_sessions ADD COLUMN IF NOT EXISTS device_type TEXT NOT NULL DEFAULT 'desktop';
		`,
	},
}

func (cs *CommonStore) migrate() error {
	// Bootstrap: create the migrations tracking table
	const bootstrap = `
	CREATE SCHEMA IF NOT EXISTS common;
	CREATE TABLE IF NOT EXISTS common.schema_migrations (
		version     INTEGER PRIMARY KEY,
		description TEXT NOT NULL DEFAULT '',
		applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := cs.db.Exec(bootstrap); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Determine which migrations have already been applied
	applied := map[int]bool{}
	rows, err := cs.db.Query("SELECT version FROM common.schema_migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return err
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Run pending migrations in order
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		log.Printf("  → Running migration %d: %s", m.Version, m.Description)

		tx, err := cs.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.Version, err)
		}

		if _, err := tx.Exec(m.SQL); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", m.Version, err)
		}

		if _, err := tx.Exec(
			"INSERT INTO common.schema_migrations (version, description) VALUES ($1, $2)",
			m.Version, m.Description,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.Version, err)
		}
	}

	return nil
}

// ─── Organizations ──────────────────────────────────────────────────────────

// CreateOrganization creates an org in the common schema and provisions its
// dedicated PostgreSQL schema with all tenant tables.
func (cs *CommonStore) CreateOrganization(ctx context.Context, org *models.Organization) error {
	if org.ID == "" {
		org.ID = uuid.New().String()
	}
	org.CreatedAt = time.Now().UTC()

	// Validate slug is not reserved
	if IsReservedSlug(org.Slug) {
		return fmt.Errorf("slug %q is reserved and cannot be used", org.Slug)
	}

	const q = `INSERT INTO common.organizations (id, name, slug, plan, created_at)
		VALUES ($1, $2, $3, $4, $5)`
	_, err := cs.db.ExecContext(ctx, q, org.ID, org.Name, org.Slug, org.Plan, org.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert org: %w", err)
	}

	// Provision the org's dedicated schema
	if err := cs.ProvisionOrgSchema(ctx, org.SchemaName()); err != nil {
		// Rollback the org record if schema creation fails
		cs.db.ExecContext(ctx, "DELETE FROM common.organizations WHERE id = $1", org.ID)
		return fmt.Errorf("provision schema: %w", err)
	}

	return nil
}

// GetOrganization returns an org by ID.
func (cs *CommonStore) GetOrganization(ctx context.Context, id string) (*models.Organization, error) {
	const q = `SELECT id, name, slug, plan, created_at FROM common.organizations WHERE id = $1`
	org := &models.Organization{}
	err := cs.db.QueryRowContext(ctx, q, id).Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return org, err
}

// GetOrgBySlug returns an org by slug.
func (cs *CommonStore) GetOrgBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	const q = `SELECT id, name, slug, plan, created_at FROM common.organizations WHERE slug = $1`
	org := &models.Organization{}
	err := cs.db.QueryRowContext(ctx, q, slug).Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return org, err
}

// ListOrganizations returns all organizations.
func (cs *CommonStore) ListOrganizations(ctx context.Context) ([]models.Organization, error) {
	const q = `SELECT id, name, slug, plan, created_at FROM common.organizations ORDER BY created_at DESC`
	rows, err := cs.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []models.Organization
	for rows.Next() {
		var org models.Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.CreatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

// ─── Users ──────────────────────────────────────────────────────────────────

// CreateUser creates a user in the common schema.
func (cs *CommonStore) CreateUser(ctx context.Context, user *models.User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}
	user.CreatedAt = time.Now().UTC()

	const q = `INSERT INTO common.users (id, email, name, avatar_url, password_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := cs.db.ExecContext(ctx, q, user.ID, user.Email, user.Name, user.AvatarURL, user.PasswordHash, user.CreatedAt)
	return err
}

// GetUser returns a user by ID.
func (cs *CommonStore) GetUser(ctx context.Context, id string) (*models.User, error) {
	const q = `SELECT id, email, name, avatar_url, password_hash, mfa_enabled, created_at FROM common.users WHERE id = $1`
	user := &models.User{}
	err := cs.db.QueryRowContext(ctx, q, id).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.PasswordHash, &user.MFAEnabled, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cs.hydrateEmailVerified(ctx, user)
	return user, nil
}

// GetUserByEmail returns a user by email (includes password hash for auth).
func (cs *CommonStore) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `SELECT id, email, name, avatar_url, password_hash, mfa_enabled, created_at FROM common.users WHERE email = $1`
	user := &models.User{}
	err := cs.db.QueryRowContext(ctx, q, email).Scan(&user.ID, &user.Email, &user.Name, &user.AvatarURL, &user.PasswordHash, &user.MFAEnabled, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cs.hydrateEmailVerified(ctx, user)
	return user, nil
}

// hydrateEmailVerified sets the computed EmailVerified field from user_emails.
func (cs *CommonStore) hydrateEmailVerified(ctx context.Context, user *models.User) {
	var verified bool
	err := cs.db.QueryRowContext(ctx,
		`SELECT verified FROM common.user_emails WHERE user_id = $1 AND is_primary = true LIMIT 1`,
		user.ID).Scan(&verified)
	if err == nil {
		user.EmailVerified = verified
	}
}

// ─── Memberships ────────────────────────────────────────────────────────────

// AddOrgMember adds a user to an organization.
func (cs *CommonStore) AddOrgMember(ctx context.Context, orgID, userID, role string) error {
	const q = `INSERT INTO common.org_members (org_id, user_id, role) VALUES ($1, $2, $3)
		ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`
	_, err := cs.db.ExecContext(ctx, q, orgID, userID, role)
	return err
}

// GetUserOrgs returns all organizations a user belongs to, with their role.
func (cs *CommonStore) GetUserOrgs(ctx context.Context, userID string) ([]models.Organization, error) {
	const q = `SELECT o.id, o.name, o.slug, o.plan, o.created_at
		FROM common.organizations o
		JOIN common.org_members m ON m.org_id = o.id
		WHERE m.user_id = $1
		ORDER BY o.name`
	rows, err := cs.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orgs []models.Organization
	for rows.Next() {
		var org models.Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.CreatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, org)
	}
	return orgs, rows.Err()
}

// IsOrgMember checks if a user belongs to an org.
func (cs *CommonStore) IsOrgMember(ctx context.Context, orgID, userID string) (bool, error) {
	const q = `SELECT 1 FROM common.org_members WHERE org_id = $1 AND user_id = $2`
	var one int
	err := cs.db.QueryRowContext(ctx, q, orgID, userID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// GetMemberRole returns the user's role in an org ("owner", "admin", "member", "viewer").
// Returns "" if the user is not a member.
func (cs *CommonStore) GetMemberRole(ctx context.Context, orgID, userID string) (string, error) {
	const q = `SELECT role FROM common.org_members WHERE org_id = $1 AND user_id = $2`
	var role string
	err := cs.db.QueryRowContext(ctx, q, orgID, userID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

// MemberInfo holds user info + role for listing.
type MemberInfo struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
}

// ListOrgMembers returns all members of an org with their user info and role.
func (cs *CommonStore) ListOrgMembers(ctx context.Context, orgID string) ([]MemberInfo, error) {
	const q = `SELECT u.id, u.email, u.name, u.avatar_url, m.role
		FROM common.users u
		JOIN common.org_members m ON m.user_id = u.id
		WHERE m.org_id = $1
		ORDER BY m.role, u.name`
	rows, err := cs.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []MemberInfo
	for rows.Next() {
		var m MemberInfo
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.AvatarURL, &m.Role); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// RemoveOrgMember removes a user from an organization.
func (cs *CommonStore) RemoveOrgMember(ctx context.Context, orgID, userID string) error {
	_, err := cs.db.ExecContext(ctx, "DELETE FROM common.org_members WHERE org_id = $1 AND user_id = $2", orgID, userID)
	return err
}

// ─── Schema Provisioning ────────────────────────────────────────────────────

// schemaNamePattern validates schema names to prevent SQL injection.
// Only allows: org_ followed by lowercase hex characters (UUID without hyphens).
var schemaNamePattern = regexp.MustCompile(`^org_[a-f0-9]{32}$`)

// ProvisionOrgSchema creates the dedicated schema for an org and runs DDL.
func (cs *CommonStore) ProvisionOrgSchema(ctx context.Context, schemaName string) error {
	if !schemaNamePattern.MatchString(schemaName) {
		return fmt.Errorf("invalid schema name: %s", schemaName)
	}

	// Schema names cannot be parameterized — validate strictly above
	ddl := fmt.Sprintf(`
	CREATE SCHEMA IF NOT EXISTS %[1]s;

	CREATE TABLE IF NOT EXISTS %[1]s.projects (
		id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name       TEXT NOT NULL,
		slug       TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(slug)
	);

	CREATE TABLE IF NOT EXISTS %[1]s.scans (
		id              TEXT PRIMARY KEY,
		project_id      UUID REFERENCES %[1]s.projects(id) ON DELETE SET NULL,
		name            TEXT NOT NULL DEFAULT '',
		target_type     TEXT NOT NULL DEFAULT 'path',
		target_path     TEXT NOT NULL DEFAULT '',
		target_url      TEXT NOT NULL DEFAULT '',
		target_ref      TEXT NOT NULL DEFAULT '',
		persona         TEXT NOT NULL DEFAULT 'sharingan',
		mode            TEXT NOT NULL DEFAULT 'direct',
		status          TEXT NOT NULL DEFAULT 'pending',
		prompt          TEXT NOT NULL DEFAULT '',
		agent_pid       INTEGER NOT NULL DEFAULT 0,
		conversation_id TEXT NOT NULL DEFAULT '',
		workspace_path  TEXT NOT NULL DEFAULT '',
		finding_count   INTEGER NOT NULL DEFAULT 0,
		sum_total       INTEGER NOT NULL DEFAULT 0,
		sum_critical    INTEGER NOT NULL DEFAULT 0,
		sum_high        INTEGER NOT NULL DEFAULT 0,
		sum_medium      INTEGER NOT NULL DEFAULT 0,
		sum_low         INTEGER NOT NULL DEFAULT 0,
		sum_info        INTEGER NOT NULL DEFAULT 0,
		error_message   TEXT NOT NULL DEFAULT '',
		created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		started_at      TIMESTAMPTZ,
		completed_at    TIMESTAMPTZ
	);

	CREATE TABLE IF NOT EXISTS %[1]s.findings (
		id          TEXT PRIMARY KEY,
		scan_id     TEXT NOT NULL DEFAULT '',
		project_id  UUID,
		fingerprint TEXT NOT NULL DEFAULT '',
		title       TEXT NOT NULL DEFAULT '',
		severity    TEXT NOT NULL DEFAULT 'info',
		cwe         TEXT NOT NULL DEFAULT '',
		owasp       TEXT NOT NULL DEFAULT '',
		cve         TEXT NOT NULL DEFAULT '',
		cvss_score  REAL NOT NULL DEFAULT 0,
		cvss_vector TEXT NOT NULL DEFAULT '',
		file        TEXT NOT NULL DEFAULT '',
		line        INTEGER NOT NULL DEFAULT 0,
		status      TEXT NOT NULL DEFAULT 'open',
		description TEXT NOT NULL DEFAULT '',
		source      TEXT NOT NULL DEFAULT '',
		seen_count  INTEGER NOT NULL DEFAULT 1,
		last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_%[1]s_findings_scan     ON %[1]s.findings(scan_id);
	CREATE INDEX IF NOT EXISTS idx_%[1]s_findings_severity ON %[1]s.findings(severity);
	CREATE INDEX IF NOT EXISTS idx_%[1]s_findings_status   ON %[1]s.findings(status);
	CREATE INDEX IF NOT EXISTS idx_%[1]s_findings_project  ON %[1]s.findings(project_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_%[1]s_findings_fp ON %[1]s.findings(fingerprint) WHERE fingerprint != '';

	CREATE TABLE IF NOT EXISTS %[1]s.exploits (
		id         TEXT PRIMARY KEY,
		finding_id TEXT NOT NULL REFERENCES %[1]s.findings(id) ON DELETE CASCADE,
		filename   TEXT NOT NULL DEFAULT '',
		language   TEXT NOT NULL DEFAULT '',
		code       TEXT NOT NULL DEFAULT '',
		validated  BOOLEAN NOT NULL DEFAULT FALSE
	);
	CREATE INDEX IF NOT EXISTS idx_%[1]s_exploits_finding ON %[1]s.exploits(finding_id);

	CREATE TABLE IF NOT EXISTS %[1]s.api_tokens (
		id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		project_id  UUID,
		name        TEXT NOT NULL DEFAULT '',
		token_hash  TEXT NOT NULL,
		prefix      TEXT NOT NULL DEFAULT '',
		created_by  UUID,
		last_used   TIMESTAMPTZ,
		expires_at  TIMESTAMPTZ,
		revoked     BOOLEAN NOT NULL DEFAULT FALSE,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_%[1]s_tokens_prefix ON %[1]s.api_tokens(prefix);
	`, schemaName)

	_, err := cs.db.ExecContext(ctx, ddl)
	return err
}

// DropOrgSchema drops an org's schema and all its data.
func (cs *CommonStore) DropOrgSchema(ctx context.Context, schemaName string) error {
	if !schemaNamePattern.MatchString(schemaName) {
		return fmt.Errorf("invalid schema name: %s", schemaName)
	}

	// Also remove the org record from common schema
	_, err := cs.db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schemaName))
	return err
}

// ─── Feature Flags ──────────────────────────────────────────────────────────

// FeatureFlag represents a feature toggle.
type FeatureFlag struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}

// IsFeatureEnabled checks if a feature flag is enabled.
func (cs *CommonStore) IsFeatureEnabled(ctx context.Context, name string) bool {
	var enabled bool
	err := cs.db.QueryRowContext(ctx, "SELECT enabled FROM common.feature_flags WHERE name = $1", name).Scan(&enabled)
	if err != nil {
		return false // default to disabled on error
	}
	return enabled
}

// ListFeatureFlags returns all feature flags.
func (cs *CommonStore) ListFeatureFlags(ctx context.Context) ([]FeatureFlag, error) {
	rows, err := cs.db.QueryContext(ctx, "SELECT name, enabled, description FROM common.feature_flags ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flags []FeatureFlag
	for rows.Next() {
		var f FeatureFlag
		if err := rows.Scan(&f.Name, &f.Enabled, &f.Description); err != nil {
			return nil, err
		}
		flags = append(flags, f)
	}
	return flags, rows.Err()
}

// SetFeatureFlag updates a feature flag's enabled state.
func (cs *CommonStore) SetFeatureFlag(ctx context.Context, name string, enabled bool) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.feature_flags SET enabled = $1, updated_at = NOW() WHERE name = $2",
		enabled, name)
	return err
}

// ─── App Config ─────────────────────────────────────────────────────────────

// GetConfig returns a config value by key.
func (cs *CommonStore) GetConfig(ctx context.Context, key string) (string, error) {
	var val string
	err := cs.db.QueryRowContext(ctx, "SELECT value FROM common.app_config WHERE key = $1", key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

// SetConfig sets a config key/value pair (upsert).
func (cs *CommonStore) SetConfig(ctx context.Context, key, value string) error {
	_, err := cs.db.ExecContext(ctx,
		`INSERT INTO common.app_config (key, value, updated_at) VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
		key, value)
	return err
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// DB returns the underlying database connection for use by the tenant store.
func (cs *CommonStore) DB() *sql.DB {
	return cs.db
}

// SanitizeSlug normalizes a slug for use as an org identifier.
func SanitizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var clean []byte
	for i := range s {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			clean = append(clean, c)
		}
	}
	return string(clean)
}

// reservedSlugs is a blacklist of subdomain names that cannot be used as org slugs.
var reservedSlugs = map[string]bool{
	"admin": true, "dashboard": true, "api": true, "app": true,
	"www": true, "mail": true, "smtp": true, "ftp": true,
	"auth": true, "login": true, "register": true, "signup": true,
	"billing": true, "payment": true, "support": true, "help": true,
	"docs": true, "blog": true, "status": true, "health": true,
	"metrics": true, "monitoring": true, "grafana": true, "prometheus": true,
	"static": true, "assets": true, "cdn": true, "media": true,
	"root": true, "system": true, "internal": true, "private": true,
	"aegis": true, "agent": true, "scanner": true, "security": true,
	"test": true, "staging": true, "dev": true, "prod": true,
	"console": true, "portal": true, "account": true, "settings": true,
	"webhook": true, "webhooks": true, "callback": true, "oauth": true,
}

// IsReservedSlug checks if a slug is in the reserved blacklist.
func IsReservedSlug(slug string) bool {
	return reservedSlugs[strings.ToLower(slug)]
}

// GetOrgByDomain returns an org by its custom domain.
func (cs *CommonStore) GetOrgByDomain(ctx context.Context, domain string) (*models.Organization, error) {
	const q = `SELECT id, name, slug, plan, created_at FROM common.organizations WHERE custom_domain = $1`
	org := &models.Organization{}
	err := cs.db.QueryRowContext(ctx, q, domain).Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return org, err
}

// ─── Password Reset ─────────────────────────────────────────────────────────

// PasswordResetToken represents a stored password reset token.
type PasswordResetToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// CreatePasswordResetToken inserts a new password reset token.
func (cs *CommonStore) CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	const q = `INSERT INTO common.password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`
	_, err := cs.db.ExecContext(ctx, q, userID, tokenHash, expiresAt)
	return err
}

// GetPasswordResetToken returns a token by hash that hasn't been used and hasn't expired.
func (cs *CommonStore) GetPasswordResetToken(ctx context.Context, tokenHash string) (*PasswordResetToken, error) {
	const q = `SELECT id, user_id, token_hash, expires_at, used_at, created_at
		FROM common.password_reset_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()`
	t := &PasswordResetToken{}
	err := cs.db.QueryRowContext(ctx, q, tokenHash).Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

// MarkResetTokenUsed marks a reset token as used.
func (cs *CommonStore) MarkResetTokenUsed(ctx context.Context, id string) error {
	_, err := cs.db.ExecContext(ctx, "UPDATE common.password_reset_tokens SET used_at = NOW() WHERE id = $1", id)
	return err
}

// InvalidateResetTokens marks all unused tokens for a user as used.
func (cs *CommonStore) InvalidateResetTokens(ctx context.Context, userID string) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.password_reset_tokens SET used_at = NOW() WHERE user_id = $1 AND used_at IS NULL",
		userID)
	return err
}

// UpdateUserPassword updates a user's password hash.
func (cs *CommonStore) UpdateUserPassword(ctx context.Context, userID, newHash string) error {
	_, err := cs.db.ExecContext(ctx, "UPDATE common.users SET password_hash = $1 WHERE id = $2", newHash, userID)
	return err
}

// ─── User Emails ────────────────────────────────────────────────────────────

// ListUserEmails returns all email addresses for a user.
func (cs *CommonStore) ListUserEmails(ctx context.Context, userID string) ([]models.UserEmail, error) {
	const q = `SELECT id, user_id, email, is_primary, verified, verified_at, created_at
		FROM common.user_emails WHERE user_id = $1 ORDER BY is_primary DESC, created_at ASC`
	rows, err := cs.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.UserEmail
	for rows.Next() {
		var e models.UserEmail
		if err := rows.Scan(&e.ID, &e.UserID, &e.Email, &e.IsPrimary, &e.Verified, &e.VerifiedAt, &e.CreatedAt); err != nil {
			return nil, err
		}
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

// AddUserEmail adds a new (unverified) email address for a user.
func (cs *CommonStore) AddUserEmail(ctx context.Context, userID, email string) (*models.UserEmail, error) {
	e := &models.UserEmail{}
	const q = `INSERT INTO common.user_emails (user_id, email) VALUES ($1, $2)
		RETURNING id, user_id, email, is_primary, verified, verified_at, created_at`
	err := cs.db.QueryRowContext(ctx, q, userID, email).Scan(
		&e.ID, &e.UserID, &e.Email, &e.IsPrimary, &e.Verified, &e.VerifiedAt, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// AddPrimaryUserEmail adds a primary email for a user (used during registration).
func (cs *CommonStore) AddPrimaryUserEmail(ctx context.Context, userID, email string) (*models.UserEmail, error) {
	e := &models.UserEmail{}
	const q = `INSERT INTO common.user_emails (user_id, email, is_primary) VALUES ($1, $2, TRUE)
		RETURNING id, user_id, email, is_primary, verified, verified_at, created_at`
	err := cs.db.QueryRowContext(ctx, q, userID, email).Scan(
		&e.ID, &e.UserID, &e.Email, &e.IsPrimary, &e.Verified, &e.VerifiedAt, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// RemoveUserEmail deletes a non-primary email.
func (cs *CommonStore) RemoveUserEmail(ctx context.Context, emailID, userID string) error {
	result, err := cs.db.ExecContext(ctx,
		"DELETE FROM common.user_emails WHERE id = $1 AND user_id = $2 AND is_primary = FALSE",
		emailID, userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("cannot remove primary email or email not found")
	}
	return nil
}

// SetPrimaryEmail sets an email as primary, demoting the current primary.
// Also updates the denormalized email on the users table.
func (cs *CommonStore) SetPrimaryEmail(ctx context.Context, emailID, userID string) error {
	tx, err := cs.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Demote current primary
	_, err = tx.ExecContext(ctx,
		"UPDATE common.user_emails SET is_primary = FALSE WHERE user_id = $1 AND is_primary = TRUE",
		userID)
	if err != nil {
		return err
	}

	// Promote new primary
	var newEmail string
	err = tx.QueryRowContext(ctx,
		"UPDATE common.user_emails SET is_primary = TRUE WHERE id = $1 AND user_id = $2 RETURNING email",
		emailID, userID).Scan(&newEmail)
	if err != nil {
		return fmt.Errorf("email not found")
	}

	// Update denormalized email on users table
	_, err = tx.ExecContext(ctx,
		"UPDATE common.users SET email = $1 WHERE id = $2",
		newEmail, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetUserEmailByAddress looks up an email record by address.
func (cs *CommonStore) GetUserEmailByAddress(ctx context.Context, email string) (*models.UserEmail, error) {
	e := &models.UserEmail{}
	const q = `SELECT id, user_id, email, is_primary, verified, verified_at, created_at
		FROM common.user_emails WHERE email = $1`
	err := cs.db.QueryRowContext(ctx, q, email).Scan(
		&e.ID, &e.UserID, &e.Email, &e.IsPrimary, &e.Verified, &e.VerifiedAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// MarkUserEmailVerified marks an email as verified.
func (cs *CommonStore) MarkUserEmailVerified(ctx context.Context, emailID string) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.user_emails SET verified = TRUE, verified_at = NOW() WHERE id = $1",
		emailID)
	return err
}

// GetUserEmail returns a single email by ID and user.
func (cs *CommonStore) GetUserEmail(ctx context.Context, emailID, userID string) (*models.UserEmail, error) {
	e := &models.UserEmail{}
	const q = `SELECT id, user_id, email, is_primary, verified, verified_at, created_at
		FROM common.user_emails WHERE id = $1 AND user_id = $2`
	err := cs.db.QueryRowContext(ctx, q, emailID, userID).Scan(
		&e.ID, &e.UserID, &e.Email, &e.IsPrimary, &e.Verified, &e.VerifiedAt, &e.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return e, err
}

// HasVerifiedEmail checks if a user has at least one verified email.
func (cs *CommonStore) HasVerifiedEmail(ctx context.Context, userID string) (bool, error) {
	var count int
	err := cs.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM common.user_emails WHERE user_id = $1 AND verified = TRUE",
		userID).Scan(&count)
	return count > 0, err
}

// ─── MFA Devices ────────────────────────────────────────────────────────────

// ListMFADevices returns all MFA devices for a user.
func (cs *CommonStore) ListMFADevices(ctx context.Context, userID string) ([]models.MFADevice, error) {
	const q = `SELECT id, user_id, name, type, secret, email, verified, verified_at, last_used_at, created_at
		FROM common.mfa_devices WHERE user_id = $1 ORDER BY created_at ASC`
	rows, err := cs.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.MFADevice
	for rows.Next() {
		var d models.MFADevice
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Type, &d.Secret, &d.Email,
			&d.Verified, &d.VerifiedAt, &d.LastUsedAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// GetMFADevice returns a single MFA device.
func (cs *CommonStore) GetMFADevice(ctx context.Context, deviceID, userID string) (*models.MFADevice, error) {
	d := &models.MFADevice{}
	const q = `SELECT id, user_id, name, type, secret, email, verified, verified_at, last_used_at, created_at
		FROM common.mfa_devices WHERE id = $1 AND user_id = $2`
	err := cs.db.QueryRowContext(ctx, q, deviceID, userID).Scan(
		&d.ID, &d.UserID, &d.Name, &d.Type, &d.Secret, &d.Email,
		&d.Verified, &d.VerifiedAt, &d.LastUsedAt, &d.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return d, err
}

// CreateMFADevice inserts a new MFA device.
func (cs *CommonStore) CreateMFADevice(ctx context.Context, d *models.MFADevice) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	d.CreatedAt = time.Now().UTC()
	const q = `INSERT INTO common.mfa_devices (id, user_id, name, type, secret, email, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := cs.db.ExecContext(ctx, q, d.ID, d.UserID, d.Name, d.Type, d.Secret, d.Email, d.CreatedAt)
	return err
}

// VerifyMFADevice marks a device as verified.
func (cs *CommonStore) VerifyMFADevice(ctx context.Context, deviceID string) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.mfa_devices SET verified = TRUE, verified_at = NOW() WHERE id = $1",
		deviceID)
	return err
}

// RemoveMFADevice deletes an MFA device.
func (cs *CommonStore) RemoveMFADevice(ctx context.Context, deviceID, userID string) error {
	_, err := cs.db.ExecContext(ctx,
		"DELETE FROM common.mfa_devices WHERE id = $1 AND user_id = $2",
		deviceID, userID)
	return err
}

// UpdateMFADeviceLastUsed touches the last_used_at timestamp.
func (cs *CommonStore) UpdateMFADeviceLastUsed(ctx context.Context, deviceID string) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.mfa_devices SET last_used_at = NOW() WHERE id = $1",
		deviceID)
	return err
}

// GetVerifiedMFADevices returns only verified devices for a user (for login flow).
func (cs *CommonStore) GetVerifiedMFADevices(ctx context.Context, userID string) ([]models.MFADevice, error) {
	const q = `SELECT id, user_id, name, type, secret, email, verified, verified_at, last_used_at, created_at
		FROM common.mfa_devices WHERE user_id = $1 AND verified = TRUE ORDER BY created_at ASC`
	rows, err := cs.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.MFADevice
	for rows.Next() {
		var d models.MFADevice
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Type, &d.Secret, &d.Email,
			&d.Verified, &d.VerifiedAt, &d.LastUsedAt, &d.CreatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, rows.Err()
}

// HasVerifiedMFADevice checks if a user has at least one verified MFA device.
func (cs *CommonStore) HasVerifiedMFADevice(ctx context.Context, userID string) (bool, error) {
	var count int
	err := cs.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM common.mfa_devices WHERE user_id = $1 AND verified = TRUE",
		userID).Scan(&count)
	return count > 0, err
}

// ─── MFA Toggle ─────────────────────────────────────────────────────────────

// EnableMFA sets mfa_enabled = TRUE on the users table.
func (cs *CommonStore) EnableMFA(ctx context.Context, userID string) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.users SET mfa_enabled = TRUE WHERE id = $1",
		userID)
	return err
}

// DisableMFA sets mfa_enabled = FALSE and deletes recovery codes.
func (cs *CommonStore) DisableMFA(ctx context.Context, userID string) error {
	tx, err := cs.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		"UPDATE common.users SET mfa_enabled = FALSE WHERE id = $1", userID)
	if err != nil {
		return err
	}

	// Delete recovery codes
	_, err = tx.ExecContext(ctx, "DELETE FROM common.mfa_recovery_codes WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ─── Recovery Codes ─────────────────────────────────────────────────────────

// CreateRecoveryCodes bulk-inserts hashed recovery codes for a user.
// Deletes any existing codes first.
func (cs *CommonStore) CreateRecoveryCodes(ctx context.Context, userID string, codeHashes []string) error {
	tx, err := cs.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM common.mfa_recovery_codes WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	const q = `INSERT INTO common.mfa_recovery_codes (user_id, code_hash) VALUES ($1, $2)`
	for _, hash := range codeHashes {
		if _, err := tx.ExecContext(ctx, q, userID, hash); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteRecoveryCodes removes all recovery codes for a user.
func (cs *CommonStore) DeleteRecoveryCodes(ctx context.Context, userID string) error {
	_, err := cs.db.ExecContext(ctx,
		"DELETE FROM common.mfa_recovery_codes WHERE user_id = $1", userID)
	return err
}

// CountUnusedRecoveryCodes returns the number of unused recovery codes.
func (cs *CommonStore) CountUnusedRecoveryCodes(ctx context.Context, userID string) (int, error) {
	var count int
	err := cs.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM common.mfa_recovery_codes WHERE user_id = $1 AND used_at IS NULL",
		userID).Scan(&count)
	return count, err
}

// ─── Email OTP ──────────────────────────────────────────────────────────────

// CreateEmailOTP stores a hashed OTP code for email-based MFA.
func (cs *CommonStore) CreateEmailOTP(ctx context.Context, userID, deviceID, codeHash string, expiresAt time.Time) error {
	// Invalidate old codes first
	cs.InvalidateEmailOTPs(ctx, userID, deviceID)

	const q = `INSERT INTO common.email_otp_codes (user_id, device_id, code_hash, expires_at)
		VALUES ($1, $2, $3, $4)`
	_, err := cs.db.ExecContext(ctx, q, userID, deviceID, codeHash, expiresAt)
	return err
}

// ValidateEmailOTP checks if a code matches and marks it as used.
func (cs *CommonStore) ValidateEmailOTP(ctx context.Context, userID, deviceID, codeHash string) (bool, error) {
	result, err := cs.db.ExecContext(ctx,
		`UPDATE common.email_otp_codes
		 SET used_at = NOW()
		 WHERE user_id = $1 AND device_id = $2 AND code_hash = $3
		   AND used_at IS NULL AND expires_at > NOW()`,
		userID, deviceID, codeHash)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

// InvalidateEmailOTPs marks all unused OTPs for a user+device as used.
func (cs *CommonStore) InvalidateEmailOTPs(ctx context.Context, userID, deviceID string) {
	_, _ = cs.db.ExecContext(ctx,
		"UPDATE common.email_otp_codes SET used_at = NOW() WHERE user_id = $1 AND device_id = $2 AND used_at IS NULL",
		userID, deviceID)
}

// ─── User Sessions ──────────────────────────────────────────────────────────

// CreateSession inserts a new session row.
func (cs *CommonStore) CreateSession(ctx context.Context, s *models.UserSession) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	s.CreatedAt = time.Now().UTC()
	s.LastActiveAt = s.CreatedAt

	const q = `INSERT INTO common.user_sessions (id, user_id, jti, ip_address, user_agent, browser, os, device_type, created_at, last_active_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := cs.db.ExecContext(ctx, q, s.ID, s.UserID, s.JTI, s.IPAddress, s.UserAgent, s.Browser, s.OS, s.DeviceType, s.CreatedAt, s.LastActiveAt, s.ExpiresAt)
	return err
}

// ListActiveSessions returns non-revoked, non-expired sessions for a user.
func (cs *CommonStore) ListActiveSessions(ctx context.Context, userID string) ([]models.UserSession, error) {
	const q = `SELECT id, user_id, jti, ip_address, user_agent, browser, os, device_type, created_at, last_active_at, expires_at
		FROM common.user_sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > NOW()
		ORDER BY last_active_at DESC`
	rows, err := cs.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []models.UserSession
	for rows.Next() {
		var s models.UserSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.JTI, &s.IPAddress, &s.UserAgent,
			&s.Browser, &s.OS, &s.DeviceType,
			&s.CreatedAt, &s.LastActiveAt, &s.ExpiresAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

// RevokeSession revokes a specific session.
func (cs *CommonStore) RevokeSession(ctx context.Context, sessionID, userID string) error {
	result, err := cs.db.ExecContext(ctx,
		"UPDATE common.user_sessions SET revoked_at = NOW() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL",
		sessionID, userID)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

// RevokeAllSessions revokes all sessions for a user except the one with the given JTI.
func (cs *CommonStore) RevokeAllSessions(ctx context.Context, userID, exceptJTI string) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.user_sessions SET revoked_at = NOW() WHERE user_id = $1 AND jti != $2 AND revoked_at IS NULL",
		userID, exceptJTI)
	return err
}

// IsSessionRevoked checks if a session has been revoked.
func (cs *CommonStore) IsSessionRevoked(ctx context.Context, jti string) (bool, error) {
	var revokedAt sql.NullTime
	err := cs.db.QueryRowContext(ctx,
		"SELECT revoked_at FROM common.user_sessions WHERE jti = $1",
		jti).Scan(&revokedAt)
	if err == sql.ErrNoRows {
		// No session row → token predates session tracking, allow it
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return revokedAt.Valid, nil
}

// TouchSession updates the last_active_at timestamp.
func (cs *CommonStore) TouchSession(ctx context.Context, jti string) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.user_sessions SET last_active_at = NOW() WHERE jti = $1 AND revoked_at IS NULL",
		jti)
	return err
}

// RevokeSessionByJTI revokes a session by its JTI (used during logout).
func (cs *CommonStore) RevokeSessionByJTI(ctx context.Context, jti string) error {
	_, err := cs.db.ExecContext(ctx,
		"UPDATE common.user_sessions SET revoked_at = NOW() WHERE jti = $1 AND revoked_at IS NULL",
		jti)
	return err
}

