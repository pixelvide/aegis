// Package store defines the data access interface and SQLite implementation.
package store

import (
	"context"

	"github.com/pixelvide/aegis/server/internal/models"
)

// Store is the data access interface for the Aegis server.
// All methods accept a context for cancellation support.
type Store interface {
	// --- Scans ---

	// CreateScan persists a new scan.
	CreateScan(ctx context.Context, scan *models.Scan) error

	// GetScan returns a scan by ID.
	GetScan(ctx context.Context, id string) (*models.Scan, error)

	// ListScans returns all scans, newest first.
	ListScans(ctx context.Context, projectID string) ([]models.Scan, error)

	// UpdateScan updates a scan's mutable fields (status, pid, timestamps, summary).
	UpdateScan(ctx context.Context, scan *models.Scan) error

	// DeleteScan removes a scan and all its findings.
	DeleteScan(ctx context.Context, id string) error

	// --- Findings ---

	// CreateFinding persists a new finding.
	CreateFinding(ctx context.Context, finding *models.Finding) error

	// UpsertFinding inserts a new finding or updates an existing one
	// if a finding with the same fingerprint already exists.
	// Returns true if an existing finding was updated (deduplicated).
	UpsertFinding(ctx context.Context, finding *models.Finding) (bool, error)

	// GetFinding returns a finding by ID.
	GetFinding(ctx context.Context, id string) (*models.Finding, error)

	// ListFindings returns findings, optionally filtered.
	ListFindings(ctx context.Context, filter FindingFilter) ([]models.Finding, error)

	// UpdateFindingStatus updates a finding's triage status.
	UpdateFindingStatus(ctx context.Context, id string, status models.FindingStatus) error

	// --- Exploits ---

	// CreateExploit persists an exploit attached to a finding.
	CreateExploit(ctx context.Context, exploit *models.Exploit) error

	// ListExploits returns all exploits for a finding.
	ListExploits(ctx context.Context, findingID string) ([]models.Exploit, error)

	// GetExploit returns a single exploit by ID.
	GetExploit(ctx context.Context, id string) (*models.Exploit, error)

	// --- API Tokens ---

	// CreateAPIToken persists a new API token (hash only, never plaintext).
	CreateAPIToken(ctx context.Context, token *models.APIToken, hash string) error

	// GetAPITokenByPrefix returns a token and its hash for verification.
	GetAPITokenByPrefix(ctx context.Context, prefix string) (*models.APIToken, string, error)

	// ListAPITokens returns all non-revoked tokens (without hashes).
	ListAPITokens(ctx context.Context) ([]models.APIToken, error)

	// RevokeAPIToken soft-deletes a token by setting revoked=true.
	RevokeAPIToken(ctx context.Context, id string) error

	// UpdateTokenLastUsed updates the last_used timestamp.
	UpdateTokenLastUsed(ctx context.Context, id string) error

	// --- Dashboard ---

	// GetDashboardStats returns aggregate statistics.
	GetDashboardStats(ctx context.Context, projectID string) (*DashboardStats, error)

	// --- Projects ---

	// CreateProject persists a new project.
	CreateProject(ctx context.Context, project *models.Project) error

	// ListProjects returns all projects in this org.
	ListProjects(ctx context.Context) ([]models.Project, error)

	// GetProjectBySlug returns a project by slug.
	GetProjectBySlug(ctx context.Context, slug string) (*models.Project, error)

	// --- Lifecycle ---

	// --- Org Feature Flags ---

	// IsOrgFeatureActive returns true only if the flag exists AND is both provisioned and enabled.
	IsOrgFeatureActive(ctx context.Context, name string) bool

	// ListOrgFeatureFlags returns all org-level feature flags.
	ListOrgFeatureFlags(ctx context.Context) ([]OrgFeatureFlag, error)

	// SetOrgFeatureEnabled updates the enabled state of an org feature flag.
	// Returns an error if the flag does not exist or is not provisioned.
	SetOrgFeatureEnabled(ctx context.Context, name string, enabled bool) error

	// ListAPITokensByProject returns all non-revoked tokens for a specific project.
	ListAPITokensByProject(ctx context.Context, projectID string) ([]models.APIToken, error)

	// GetAPIToken returns a single token by ID (without hash).
	GetAPIToken(ctx context.Context, id string) (*models.APIToken, error)

	// Close releases database resources.
	Close() error
}

// FindingFilter controls which findings are returned by ListFindings.
type FindingFilter struct {
	ScanID    string
	ProjectID string
	Severity  string
	Status    string
	CWE       string
}

// DashboardStats is an aggregate summary for the dashboard page.
type DashboardStats struct {
	TotalScans       int             `json:"total_scans"`
	ActiveScans      int             `json:"active_scans"`
	TotalFindings    int             `json:"total_findings"`
	SeverityBreakdown models.Summary `json:"severity_breakdown"`
	RecentFindings   []models.Finding `json:"recent_findings"`
}
