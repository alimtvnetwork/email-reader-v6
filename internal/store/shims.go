// shims.go exposes typed query methods that wrap the package-private
// `*sql.DB` so feature backends never touch `database/sql` directly.
//
// Why this file exists: AC-DB-52 (`Test_AST_CoreUsesStoreOnly`) requires
// that `internal/core/*` not import `database/sql`. The pre-shim
// callers (`internal/core/tools_export.go`, `tools_diagnose.go`,
// `internal/exporter/exporter.go`) reached through the exported
// `Store.DB` field; this file replaces those reach-throughs with three
// typed methods:
//
//   - `Store.QueryEmailExportRows`  — the streaming SELECT for ExportCsv
//   - `Store.CountEmails`           — the matching COUNT(*) for the
//                                     PhaseCounting tick
//   - `Store.QueryOpenedUrls`       — the Delta-#1 audit-row reader for
//                                     RecentOpenedUrls
//
// The methods return a `RowsScanner` (a 4-method subset of `*sql.Rows`)
// so callers can stream without taking on a `database/sql` dependency.
//
// Filter shapes (`EmailExportFilter`, `OpenedUrlListFilter`) live in
// this package — primitive fields only, no enums imported from `core`,
// to keep the dependency direction one-way (`core` → `store`).
//
// Spec: spec/23-app-database/97-acceptance-criteria.md §F (AC-DB-50…52).
package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/lovable/email-read/internal/errtrace"
)

// RowsScanner is the slim subset of `*sql.Rows` that streaming callers
// need. Defined here so callers can range over results without
// importing `database/sql`.
type RowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

// Compile-time confirmation `*sql.Rows` satisfies RowsScanner.
var _ RowsScanner = (*sql.Rows)(nil)

// EmailExportFilter mirrors the user-facing filter knobs of ExportCsv.
// Empty Alias means "all aliases"; zero Since/Until means no bound.
type EmailExportFilter struct {
	Alias string
	Since time.Time
	Until time.Time
}

// emailExportColumns is the explicit PascalCase column list shared by
// QueryEmailExportRows and CountEmails. No `SELECT *` (AC-DB-D-04 spirit).
const emailExportColumns = `Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
                              Subject, BodyText, BodyHtml, ReceivedAt, FilePath, CreatedAt`

// QueryEmailExportRows streams the Emails table filtered per spec.
// Caller is responsible for `defer rows.Close()`.
func (s *Store) QueryEmailExportRows(ctx context.Context, f EmailExportFilter) (RowsScanner, error) {
	q, args := buildEmailExportQuery(f, false)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, errtrace.Wrap(err, "QueryEmailExportRows")
	}
	return rows, nil
}

// CountEmails returns the row count matching the same filter shape used
// by QueryEmailExportRows, so PhaseCounting and PhaseWriting agree.
func (s *Store) CountEmails(ctx context.Context, f EmailExportFilter) (int, error) {
	q, args := buildEmailExportQuery(f, true)
	var n int
	if err := s.DB.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, errtrace.Wrap(err, "CountEmails")
	}
	return n, nil
}

// buildEmailExportQuery composes the SELECT (or COUNT) + bound args.
// `count=true` swaps the projection for `COUNT(*)` and drops ORDER BY.
// Bind parameters are appended in lock-step with the WHERE clauses
// (injection-safe by construction).
func buildEmailExportQuery(f EmailExportFilter, count bool) (string, []any) {
	var sb strings.Builder
	if count {
		sb.WriteString(`SELECT COUNT(*) FROM Emails`)
	} else {
		sb.WriteString(`SELECT `)
		sb.WriteString(emailExportColumns)
		sb.WriteString(` FROM Emails`)
	}
	where, args := whereForEmailExport(f)
	if where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(where)
	}
	if !count {
		sb.WriteString(" ORDER BY Id ASC")
	}
	return sb.String(), args
}

func whereForEmailExport(f EmailExportFilter) (string, []any) {
	var clauses []string
	var args []any
	if f.Alias != "" {
		clauses = append(clauses, "Alias = ?")
		args = append(args, f.Alias)
	}
	if !f.Since.IsZero() {
		clauses = append(clauses, "ReceivedAt >= ?")
		args = append(args, f.Since.UTC())
	}
	if !f.Until.IsZero() {
		clauses = append(clauses, "ReceivedAt < ?")
		args = append(args, f.Until.UTC())
	}
	return strings.Join(clauses, " AND "), args
}

// OpenedUrlListFilter mirrors the user-facing filter knobs of
// RecentOpenedUrls. Limit must be > 0; Before is required (zero ⇒ now).
// Origin is a free-form string here (not an enum) so this package never
// has to import the `core.OpenUrlOrigin` type.
type OpenedUrlListFilter struct {
	Before time.Time
	Alias  string
	Origin string
	Limit  int
}

// openedUrlAuditColumns is the explicit Delta-#1 column list. Order
// must match `core.scanOpenedUrlRows`.
const openedUrlAuditColumns = `Id, EmailId, Alias, RuleName, Origin, Url,
                                 OriginalUrl, IsDeduped, IsIncognito, TraceId, OpenedAt`

// QueryOpenedUrls streams the OpenedUrls audit table filtered per spec.
// Caller is responsible for `defer rows.Close()`.
func (s *Store) QueryOpenedUrls(ctx context.Context, f OpenedUrlListFilter) (RowsScanner, error) {
	q, args := buildOpenedUrlsQuery(f)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, errtrace.Wrap(err, "QueryOpenedUrls")
	}
	return rows, nil
}

func buildOpenedUrlsQuery(f OpenedUrlListFilter) (string, []any) {
	var sb strings.Builder
	sb.WriteString(`SELECT `)
	sb.WriteString(openedUrlAuditColumns)
	sb.WriteString(` FROM OpenedUrls WHERE OpenedAt < ?`)
	args := []any{f.Before}
	if f.Alias != "" {
		sb.WriteString(" AND Alias = ?")
		args = append(args, f.Alias)
	}
	if f.Origin != "" {
		sb.WriteString(" AND Origin = ?")
		args = append(args, f.Origin)
	}
	sb.WriteString(" ORDER BY OpenedAt DESC, Id DESC LIMIT ?")
	args = append(args, f.Limit)
	return sb.String(), args
}
