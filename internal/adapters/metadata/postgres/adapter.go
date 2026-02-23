package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SecDuckOps/agent/internal/domain"
	"github.com/SecDuckOps/agent/internal/ports"
	"github.com/SecDuckOps/shared/types"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Adapter implements ports.MetadataDB using PostgreSQL.
// Stores vulnerability metadata: scan results, vulnerabilities, status transitions.
type Adapter struct {
	db *sql.DB
}

// Config holds connection parameters for the PostgreSQL adapter.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// DSN builds a PostgreSQL connection string.
func (c Config) DSN() string {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, sslMode,
	)
}

// NewAdapter creates a new PostgreSQL adapter and verifies the connection.
func NewAdapter(cfg Config) (*Adapter, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: failed to open connection")
	}

	// Connection pool tuning
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: failed to ping")
	}

	return &Adapter{db: db}, nil
}

// Migrate creates the required tables if they don't exist.
func (a *Adapter) Migrate(ctx context.Context) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS scan_results (
			scan_id       TEXT PRIMARY KEY,
			scanner_type  TEXT NOT NULL,
			status        TEXT NOT NULL DEFAULT 'PENDING',
			target        TEXT,
			summary       JSONB,
			started_at    TIMESTAMPTZ,
			completed_at  TIMESTAMPTZ,
			error         TEXT,
			created_at    TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS vulnerabilities (
			id            TEXT PRIMARY KEY,
			scan_id       TEXT NOT NULL REFERENCES scan_results(scan_id) ON DELETE CASCADE,
			title         TEXT NOT NULL,
			description   TEXT,
			severity      TEXT NOT NULL,
			location      TEXT,
			line          INT DEFAULT 0,
			cve           TEXT,
			cvss          DOUBLE PRECISION DEFAULT 0,
			remediation   TEXT,
			references    JSONB,
			detected_at   TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_vulns_scan_id ON vulnerabilities(scan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_vulns_severity ON vulnerabilities(severity)`,
		`CREATE INDEX IF NOT EXISTS idx_scans_status ON scan_results(status)`,
	}

	for _, q := range queries {
		if _, err := a.db.ExecContext(ctx, q); err != nil {
			return types.Wrap(err, types.ErrCodeInternal, "postgres: migration failed")
		}
	}
	return nil
}

// SaveScanResult persists a completed scan result and its vulnerabilities.
func (a *Adapter) SaveScanResult(ctx context.Context, result *domain.ScanResult) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "postgres: begin tx")
	}
	defer tx.Rollback()

	summaryJSON, _ := json.Marshal(result.Summary)

	_, err = tx.ExecContext(ctx,
		`INSERT INTO scan_results (scan_id, scanner_type, status, summary, started_at, completed_at, error)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (scan_id) DO UPDATE SET
		   status = EXCLUDED.status,
		   summary = EXCLUDED.summary,
		   completed_at = EXCLUDED.completed_at,
		   error = EXCLUDED.error`,
		result.ScanID, result.ScannerType, result.Status,
		summaryJSON, result.StartedAt, result.CompletedAt, result.Error,
	)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "postgres: insert scan_result")
	}

	// Insert vulnerabilities
	for _, v := range result.Vulnerabilities {
		refsJSON, _ := json.Marshal(v.References)
		_, err = tx.ExecContext(ctx,
			`INSERT INTO vulnerabilities (id, scan_id, title, description, severity, location, line, cve, cvss, remediation, references, detected_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			 ON CONFLICT (id) DO NOTHING`,
			v.ID, result.ScanID, v.Title, v.Description, v.Severity,
			v.Location, v.Line, v.CVE, v.CVSS, v.Remediation, refsJSON, v.DetectedAt,
		)
		if err != nil {
			return types.Wrapf(err, types.ErrCodeInternal, "postgres: insert vulnerability %s", v.ID)
		}
	}

	return tx.Commit()
}

// GetScanResult retrieves a scan result by its ID.
func (a *Adapter) GetScanResult(ctx context.Context, scanID string) (*domain.ScanResult, error) {
	row := a.db.QueryRowContext(ctx,
		`SELECT scan_id, scanner_type, status, summary, started_at, completed_at, error
		 FROM scan_results WHERE scan_id = $1`, scanID,
	)

	var result domain.ScanResult
	var summaryJSON []byte

	err := row.Scan(
		&result.ScanID, &result.ScannerType, &result.Status,
		&summaryJSON, &result.StartedAt, &result.CompletedAt, &result.Error,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: scan scan_result")
	}

	json.Unmarshal(summaryJSON, &result.Summary)

	// Load associated vulnerabilities
	vulns, err := a.GetVulnerabilities(ctx, scanID)
	if err != nil {
		return nil, err
	}
	result.Vulnerabilities = vulns

	return &result, nil
}

// ListScanResults returns scan results matching the given filter.
func (a *Adapter) ListScanResults(ctx context.Context, filter ports.ScanResultFilter) ([]*domain.ScanResult, error) {
	query := `SELECT scan_id, scanner_type, status, summary, started_at, completed_at, error FROM scan_results WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.ScannerType != nil {
		query += fmt.Sprintf(" AND scanner_type = $%d", argIdx)
		args = append(args, *filter.ScannerType)
		argIdx++
	}
	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, *filter.Status)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
		argIdx++
	}

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: list scan_results")
	}
	defer rows.Close()

	var results []*domain.ScanResult
	for rows.Next() {
		var r domain.ScanResult
		var summaryJSON []byte
		if err := rows.Scan(
			&r.ScanID, &r.ScannerType, &r.Status,
			&summaryJSON, &r.StartedAt, &r.CompletedAt, &r.Error,
		); err != nil {
			return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: scan row")
		}
		json.Unmarshal(summaryJSON, &r.Summary)
		results = append(results, &r)
	}

	return results, rows.Err()
}

// GetVulnerabilities retrieves all vulnerabilities for a given scan ID.
func (a *Adapter) GetVulnerabilities(ctx context.Context, scanID string) ([]domain.Vulnerability, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT id, title, description, severity, location, line, cve, cvss, remediation, references, detected_at
		 FROM vulnerabilities WHERE scan_id = $1 ORDER BY detected_at`, scanID,
	)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: query vulnerabilities")
	}
	defer rows.Close()

	var vulns []domain.Vulnerability
	for rows.Next() {
		var v domain.Vulnerability
		var refsJSON []byte
		if err := rows.Scan(
			&v.ID, &v.Title, &v.Description, &v.Severity,
			&v.Location, &v.Line, &v.CVE, &v.CVSS, &v.Remediation,
			&refsJSON, &v.DetectedAt,
		); err != nil {
			return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: scan vulnerability row")
		}
		json.Unmarshal(refsJSON, &v.References)
		vulns = append(vulns, v)
	}

	return vulns, rows.Err()
}

// GetVulnerabilityByID retrieves a single vulnerability by its ID.
func (a *Adapter) GetVulnerabilityByID(ctx context.Context, vulnID string) (*domain.Vulnerability, error) {
	row := a.db.QueryRowContext(ctx,
		`SELECT id, title, description, severity, location, line, cve, cvss, remediation, references, detected_at
		 FROM vulnerabilities WHERE id = $1`, vulnID,
	)

	var v domain.Vulnerability
	var refsJSON []byte
	err := row.Scan(
		&v.ID, &v.Title, &v.Description, &v.Severity,
		&v.Location, &v.Line, &v.CVE, &v.CVSS, &v.Remediation,
		&refsJSON, &v.DetectedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: scan vulnerability")
	}
	json.Unmarshal(refsJSON, &v.References)

	return &v, nil
}

// CountBySeverity returns the count of vulnerabilities grouped by severity for a scan.
func (a *Adapter) CountBySeverity(ctx context.Context, scanID string) (map[domain.Severity]int, error) {
	rows, err := a.db.QueryContext(ctx,
		`SELECT severity, COUNT(*) FROM vulnerabilities WHERE scan_id = $1 GROUP BY severity`, scanID,
	)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: count by severity")
	}
	defer rows.Close()

	counts := make(map[domain.Severity]int)
	for rows.Next() {
		var sev domain.Severity
		var count int
		if err := rows.Scan(&sev, &count); err != nil {
			return nil, types.Wrap(err, types.ErrCodeInternal, "postgres: scan severity count")
		}
		counts[sev] = count
	}

	return counts, rows.Err()
}

// UpdateScanStatus transitions the status of a scan.
func (a *Adapter) UpdateScanStatus(ctx context.Context, scanID string, status domain.ScanStatus) error {
	res, err := a.db.ExecContext(ctx,
		`UPDATE scan_results SET status = $1 WHERE scan_id = $2`, status, scanID,
	)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "postgres: update scan status")
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return types.Newf(types.ErrCodeNotFound, "postgres: scan %s not found", scanID)
	}

	return nil
}

// Close gracefully shuts down the database connection pool.
func (a *Adapter) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}
