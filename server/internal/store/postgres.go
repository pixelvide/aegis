package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"

	"github.com/pixelvide/aegis/server/internal/models"
)

func newUUID() string { return uuid.New().String() }

// summaryField safely extracts a field from a Summary pointer.
func summaryField(s *models.Summary, fn func(*models.Summary) int) int {
	if s == nil {
		return 0
	}
	return fn(s)
}

// Postgres implements Store backed by a PostgreSQL database.
// It operates within a specific org schema set via search_path.
type Postgres struct {
	db     *sql.DB
	schema string // org schema name, e.g. "org_a1b2c3d4..."
}

// NewPostgres connects to a PostgreSQL database.
// databaseURL format: postgres://user:pass@host:port/dbname?sslmode=disable
func NewPostgres(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

// NewTenantStore creates a Store scoped to a specific org schema.
// The schema must already exist (created by CommonStore.ProvisionOrgSchema).
func NewTenantStore(db *sql.DB, schema string) *Postgres {
	return &Postgres{db: db, schema: schema}
}

// Close is a no-op for tenant stores — the shared db connection
// is managed by the CommonStore.
func (p *Postgres) Close() error {
	return nil
}

// withSchema returns a fully qualified table name.
func (p *Postgres) t(table string) string {
	return fmt.Sprintf("%s.%s", p.schema, table)
}

// ─── Scans ──────────────────────────────────────────────────────────────────

func (p *Postgres) CreateScan(ctx context.Context, scan *models.Scan) error {
	q := fmt.Sprintf(`INSERT INTO %s (
		id, name, target_type, target_path, target_url, target_ref,
		persona, mode, status, prompt, agent_pid, conversation_id,
		workspace_path, finding_count,
		sum_total, sum_critical, sum_high, sum_medium, sum_low, sum_info,
		error_message, created_at, started_at, completed_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
		p.t("scans"))

	_, err := p.db.ExecContext(ctx, q,
		scan.ID, scan.Name, scan.Target.Type, scan.Target.Path, scan.Target.URL, scan.Target.Ref,
		scan.Persona, scan.Mode, scan.Status, scan.Prompt, scan.AgentPID, scan.ConversationID,
		scan.WorkspacePath, scan.FindingCount,
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Total }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Critical }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.High }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Medium }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Low }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Info }),
		scan.ErrorMessage,
		scan.CreatedAt,
		scan.StartedAt,
		scan.CompletedAt,
	)
	return err
}

func (p *Postgres) GetScan(ctx context.Context, id string) (*models.Scan, error) {
	q := fmt.Sprintf(`SELECT
		id, name, target_type, target_path, target_url, target_ref,
		persona, mode, status, prompt, agent_pid, conversation_id,
		workspace_path, finding_count,
		sum_total, sum_critical, sum_high, sum_medium, sum_low, sum_info,
		error_message, created_at, started_at, completed_at
	FROM %s WHERE id = $1`, p.t("scans"))

	scan := &models.Scan{Summary: &models.Summary{}}
	var startedAt, completedAt sql.NullTime

	err := p.db.QueryRowContext(ctx, q, id).Scan(
		&scan.ID, &scan.Name, &scan.Target.Type, &scan.Target.Path, &scan.Target.URL, &scan.Target.Ref,
		&scan.Persona, &scan.Mode, &scan.Status, &scan.Prompt, &scan.AgentPID, &scan.ConversationID,
		&scan.WorkspacePath, &scan.FindingCount,
		&scan.Summary.Total, &scan.Summary.Critical, &scan.Summary.High,
		&scan.Summary.Medium, &scan.Summary.Low, &scan.Summary.Info,
		&scan.ErrorMessage, &scan.CreatedAt, &startedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if startedAt.Valid {
		scan.StartedAt = &startedAt.Time
	}
	if completedAt.Valid {
		scan.CompletedAt = &completedAt.Time
	}
	return scan, nil
}

func (p *Postgres) ListScans(ctx context.Context) ([]models.Scan, error) {
	q := fmt.Sprintf(`SELECT
		id, name, target_type, target_path, target_url, target_ref,
		persona, mode, status, prompt, agent_pid, conversation_id,
		workspace_path, finding_count,
		sum_total, sum_critical, sum_high, sum_medium, sum_low, sum_info,
		error_message, created_at, started_at, completed_at
	FROM %s ORDER BY created_at DESC`, p.t("scans"))

	rows, err := p.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []models.Scan
	for rows.Next() {
		var scan models.Scan
		scan.Summary = &models.Summary{}
		var startedAt, completedAt sql.NullTime

		if err := rows.Scan(
			&scan.ID, &scan.Name, &scan.Target.Type, &scan.Target.Path, &scan.Target.URL, &scan.Target.Ref,
			&scan.Persona, &scan.Mode, &scan.Status, &scan.Prompt, &scan.AgentPID, &scan.ConversationID,
			&scan.WorkspacePath, &scan.FindingCount,
			&scan.Summary.Total, &scan.Summary.Critical, &scan.Summary.High,
			&scan.Summary.Medium, &scan.Summary.Low, &scan.Summary.Info,
			&scan.ErrorMessage, &scan.CreatedAt, &startedAt, &completedAt,
		); err != nil {
			return nil, err
		}

		if startedAt.Valid {
			scan.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			scan.CompletedAt = &completedAt.Time
		}
		scans = append(scans, scan)
	}
	return scans, rows.Err()
}

func (p *Postgres) UpdateScan(ctx context.Context, scan *models.Scan) error {
	q := fmt.Sprintf(`UPDATE %s SET
		name = $1, status = $2, agent_pid = $3, conversation_id = $4,
		workspace_path = $5, finding_count = $6,
		sum_total = $7, sum_critical = $8, sum_high = $9, sum_medium = $10, sum_low = $11, sum_info = $12,
		error_message = $13, started_at = $14, completed_at = $15
	WHERE id = $16`, p.t("scans"))

	_, err := p.db.ExecContext(ctx, q,
		scan.Name, scan.Status, scan.AgentPID, scan.ConversationID,
		scan.WorkspacePath, scan.FindingCount,
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Total }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Critical }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.High }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Medium }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Low }),
		summaryField(scan.Summary, func(s *models.Summary) int { return s.Info }),
		scan.ErrorMessage,
		scan.StartedAt,
		scan.CompletedAt,
		scan.ID,
	)
	return err
}

func (p *Postgres) DeleteScan(ctx context.Context, id string) error {
	_, err := p.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = $1", p.t("scans")), id)
	return err
}

// ─── Findings ───────────────────────────────────────────────────────────────

func (p *Postgres) CreateFinding(ctx context.Context, f *models.Finding) error {
	q := fmt.Sprintf(`INSERT INTO %s (
		id, scan_id, project_id, fingerprint, title, severity, cwe, owasp,
		cve, cvss_score, cvss_vector, file, line, status, description,
		source, seen_count, last_seen_at, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`, p.t("findings"))

	_, err := p.db.ExecContext(ctx, q,
		f.ID, f.ScanID, f.ProjectID, f.Fingerprint,
		f.Title, f.Severity, f.CWE, f.OWASP,
		f.CVE, f.CVSSScore, f.CVSSVector,
		f.File, f.Line, f.Status, f.Description,
		f.Source, f.SeenCount, f.LastSeenAt,
		f.CreatedAt, f.UpdatedAt,
	)
	return err
}

func (p *Postgres) GetFinding(ctx context.Context, id string) (*models.Finding, error) {
	q := fmt.Sprintf(`SELECT
		id, scan_id, COALESCE(project_id::text, ''), fingerprint,
		title, severity, cwe, owasp,
		cve, cvss_score, cvss_vector,
		file, line, status, description,
		source, seen_count, last_seen_at,
		created_at, updated_at
	FROM %s WHERE id = $1`, p.t("findings"))

	f := &models.Finding{}
	err := p.db.QueryRowContext(ctx, q, id).Scan(
		&f.ID, &f.ScanID, &f.ProjectID, &f.Fingerprint,
		&f.Title, &f.Severity, &f.CWE, &f.OWASP,
		&f.CVE, &f.CVSSScore, &f.CVSSVector,
		&f.File, &f.Line, &f.Status, &f.Description,
		&f.Source, &f.SeenCount, &f.LastSeenAt,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (p *Postgres) ListFindings(ctx context.Context, filter FindingFilter) ([]models.Finding, error) {
	q := fmt.Sprintf(`SELECT
		id, scan_id, COALESCE(project_id::text, ''), fingerprint,
		title, severity, cwe, owasp,
		cve, cvss_score, cvss_vector,
		file, line, status, description,
		source, seen_count, last_seen_at,
		created_at, updated_at
	FROM %s WHERE 1=1`, p.t("findings"))
	var args []any
	argN := 1

	if filter.ScanID != "" {
		q += fmt.Sprintf(" AND scan_id = $%d", argN)
		args = append(args, filter.ScanID)
		argN++
	}
	if filter.ProjectID != "" {
		q += fmt.Sprintf(" AND project_id = $%d::uuid", argN)
		args = append(args, filter.ProjectID)
		argN++
	}
	if filter.Severity != "" {
		q += fmt.Sprintf(" AND severity = $%d", argN)
		args = append(args, filter.Severity)
		argN++
	}
	if filter.Status != "" {
		q += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, filter.Status)
		argN++
	}
	if filter.CWE != "" {
		q += fmt.Sprintf(" AND cwe = $%d", argN)
		args = append(args, filter.CWE)
		argN++
	}

	q += " ORDER BY created_at DESC"

	rows, err := p.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var f models.Finding
		if err := rows.Scan(
			&f.ID, &f.ScanID, &f.ProjectID, &f.Fingerprint,
			&f.Title, &f.Severity, &f.CWE, &f.OWASP,
			&f.CVE, &f.CVSSScore, &f.CVSSVector,
			&f.File, &f.Line, &f.Status, &f.Description,
			&f.Source, &f.SeenCount, &f.LastSeenAt,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

func (p *Postgres) UpdateFindingStatus(ctx context.Context, id string, status models.FindingStatus) error {
	q := fmt.Sprintf(`UPDATE %s SET status = $1, updated_at = $2 WHERE id = $3`, p.t("findings"))
	_, err := p.db.ExecContext(ctx, q, status, time.Now().UTC(), id)
	return err
}

// UpsertFinding inserts a new finding or updates if fingerprint matches.
// Returns true if the finding was deduplicated (existing record updated).
func (p *Postgres) UpsertFinding(ctx context.Context, f *models.Finding) (bool, error) {
	q := fmt.Sprintf(`
		INSERT INTO %s (
			id, scan_id, project_id, fingerprint, title, severity, cwe, owasp,
			cve, cvss_score, cvss_vector, file, line, status, description,
			source, seen_count, last_seen_at, created_at, updated_at
		) VALUES ($1,$2,$3::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,1,NOW(),$17,$18)
		ON CONFLICT (fingerprint) WHERE fingerprint != ''
		DO UPDATE SET
			seen_count = %[1]s.findings.seen_count + 1,
			last_seen_at = NOW(),
			updated_at = NOW(),
			line = EXCLUDED.line,
			source = EXCLUDED.source
		RETURNING (xmax = 0) AS is_insert
	`, p.t("findings"))

	var isInsert bool
	err := p.db.QueryRowContext(ctx, q,
		f.ID, f.ScanID, f.ProjectID, f.Fingerprint,
		f.Title, f.Severity, f.CWE, f.OWASP,
		f.CVE, f.CVSSScore, f.CVSSVector,
		f.File, f.Line, f.Status, f.Description,
		f.Source, f.CreatedAt, f.UpdatedAt,
	).Scan(&isInsert)
	if err != nil {
		return false, err
	}
	// deduplicated = NOT is_insert
	return !isInsert, nil
}

// ─── Exploits ───────────────────────────────────────────────────────────────

func (p *Postgres) CreateExploit(ctx context.Context, e *models.Exploit) error {
	q := fmt.Sprintf(`INSERT INTO %s (id, finding_id, filename, language, code, validated) VALUES ($1,$2,$3,$4,$5,$6)`, p.t("exploits"))
	_, err := p.db.ExecContext(ctx, q, e.ID, e.FindingID, e.Filename, e.Language, e.Code, e.Validated)
	return err
}

func (p *Postgres) ListExploits(ctx context.Context, findingID string) ([]models.Exploit, error) {
	q := fmt.Sprintf(`SELECT id, finding_id, filename, language, code, validated FROM %s WHERE finding_id = $1`, p.t("exploits"))
	rows, err := p.db.QueryContext(ctx, q, findingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var exploits []models.Exploit
	for rows.Next() {
		var e models.Exploit
		if err := rows.Scan(&e.ID, &e.FindingID, &e.Filename, &e.Language, &e.Code, &e.Validated); err != nil {
			return nil, err
		}
		exploits = append(exploits, e)
	}
	return exploits, rows.Err()
}

func (p *Postgres) GetExploit(ctx context.Context, id string) (*models.Exploit, error) {
	q := fmt.Sprintf(`SELECT id, finding_id, filename, language, code, validated FROM %s WHERE id = $1`, p.t("exploits"))
	var e models.Exploit
	err := p.db.QueryRowContext(ctx, q, id).Scan(&e.ID, &e.FindingID, &e.Filename, &e.Language, &e.Code, &e.Validated)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ─── Dashboard ──────────────────────────────────────────────────────────────

func (p *Postgres) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}

	p.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", p.t("scans"))).Scan(&stats.TotalScans)
	p.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE status IN ('pending', 'running')", p.t("scans"))).Scan(&stats.ActiveScans)
	p.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", p.t("findings"))).Scan(&stats.TotalFindings)

	breakdownQ := fmt.Sprintf(`SELECT
		COALESCE(SUM(CASE WHEN severity = 'critical' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN severity = 'high' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN severity = 'medium' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN severity = 'low' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN severity = 'info' THEN 1 ELSE 0 END), 0)
	FROM %s`, p.t("findings"))
	p.db.QueryRowContext(ctx, breakdownQ).Scan(
		&stats.SeverityBreakdown.Critical,
		&stats.SeverityBreakdown.High,
		&stats.SeverityBreakdown.Medium,
		&stats.SeverityBreakdown.Low,
		&stats.SeverityBreakdown.Info,
	)
	stats.SeverityBreakdown.Total = stats.TotalFindings

	recent, err := p.ListFindings(ctx, FindingFilter{})
	if err == nil && len(recent) > 10 {
		recent = recent[:10]
	}
	stats.RecentFindings = recent

	return stats, nil
}

// ─── Projects ───────────────────────────────────────────────────────────────

func (p *Postgres) CreateProject(ctx context.Context, project *models.Project) error {
	if project.ID == "" {
		project.ID = fmt.Sprintf("%s", newUUID())
	}
	project.CreatedAt = time.Now().UTC()

	q := fmt.Sprintf(`INSERT INTO %s (id, name, slug, created_at) VALUES ($1, $2, $3, $4)`, p.t("projects"))
	_, err := p.db.ExecContext(ctx, q, project.ID, project.Name, project.Slug, project.CreatedAt)
	return err
}

func (p *Postgres) ListProjects(ctx context.Context) ([]models.Project, error) {
	q := fmt.Sprintf(`SELECT id, name, slug, created_at FROM %s ORDER BY name`, p.t("projects"))
	rows, err := p.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var proj models.Project
		if err := rows.Scan(&proj.ID, &proj.Name, &proj.Slug, &proj.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, proj)
	}
	return projects, rows.Err()
}

func (p *Postgres) GetProjectBySlug(ctx context.Context, slug string) (*models.Project, error) {
	q := fmt.Sprintf(`SELECT id, name, slug, created_at FROM %s WHERE slug = $1`, p.t("projects"))
	proj := &models.Project{}
	err := p.db.QueryRowContext(ctx, q, slug).Scan(&proj.ID, &proj.Name, &proj.Slug, &proj.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return proj, nil
}

// ─── API Tokens ─────────────────────────────────────────────────────────────

func (p *Postgres) CreateAPIToken(ctx context.Context, token *models.APIToken, hash string) error {
	q := fmt.Sprintf(`INSERT INTO %s (
		id, project_id, name, token_hash, prefix, created_by, expires_at, created_at
	) VALUES ($1, $2::uuid, $3, $4, $5, $6::uuid, $7, $8)`, p.t("api_tokens"))

	var projectID, createdBy interface{}
	if token.ProjectID != "" {
		projectID = token.ProjectID
	}
	if token.CreatedBy != "" {
		createdBy = token.CreatedBy
	}

	_, err := p.db.ExecContext(ctx, q,
		token.ID, projectID, token.Name, hash, token.Prefix,
		createdBy, token.ExpiresAt, token.CreatedAt,
	)
	return err
}

func (p *Postgres) GetAPITokenByPrefix(ctx context.Context, prefix string) (*models.APIToken, string, error) {
	q := fmt.Sprintf(`SELECT
		id, COALESCE(project_id::text, ''), name, token_hash, prefix,
		COALESCE(created_by::text, ''), last_used, expires_at, revoked, created_at
	FROM %s WHERE prefix = $1 AND revoked = FALSE`, p.t("api_tokens"))

	var (
		t       models.APIToken
		hash    string
		lastUsed, expiresAt sql.NullTime
	)
	err := p.db.QueryRowContext(ctx, q, prefix).Scan(
		&t.ID, &t.ProjectID, &t.Name, &hash, &t.Prefix,
		&t.CreatedBy, &lastUsed, &expiresAt, &t.Revoked, &t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	if lastUsed.Valid {
		t.LastUsed = &lastUsed.Time
	}
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.Time
	}
	return &t, hash, nil
}

func (p *Postgres) ListAPITokens(ctx context.Context) ([]models.APIToken, error) {
	q := fmt.Sprintf(`SELECT
		id, COALESCE(project_id::text, ''), name, prefix,
		COALESCE(created_by::text, ''), last_used, expires_at, revoked, created_at
	FROM %s ORDER BY created_at DESC`, p.t("api_tokens"))

	rows, err := p.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []models.APIToken
	for rows.Next() {
		var (
			t        models.APIToken
			lastUsed, expiresAt sql.NullTime
		)
		if err := rows.Scan(
			&t.ID, &t.ProjectID, &t.Name, &t.Prefix,
			&t.CreatedBy, &lastUsed, &expiresAt, &t.Revoked, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			t.LastUsed = &lastUsed.Time
		}
		if expiresAt.Valid {
			t.ExpiresAt = &expiresAt.Time
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (p *Postgres) RevokeAPIToken(ctx context.Context, id string) error {
	q := fmt.Sprintf(`UPDATE %s SET revoked = TRUE WHERE id = $1`, p.t("api_tokens"))
	_, err := p.db.ExecContext(ctx, q, id)
	return err
}

func (p *Postgres) UpdateTokenLastUsed(ctx context.Context, id string) error {
	q := fmt.Sprintf(`UPDATE %s SET last_used = NOW() WHERE id = $1`, p.t("api_tokens"))
	_, err := p.db.ExecContext(ctx, q, id)
	return err
}

