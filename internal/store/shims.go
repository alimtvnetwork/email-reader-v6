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
	"github.com/lovable/email-read/internal/store/queries"
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

// (emailExportColumns moved to internal/store/queries; see P1.8.)

// QueryEmailExportRows streams the Emails table filtered per spec.
// Caller is responsible for `defer rows.Close()`.
func (s *Store) QueryEmailExportRows(ctx context.Context, f EmailExportFilter) (RowsScanner, error) {
	q, args := queries.EmailExport(filterToExportInput(f))
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, errtrace.Wrap(err, "QueryEmailExportRows")
	}
	return rows, nil
}

// CountEmailsFiltered returns the row count matching the same filter
// shape used by QueryEmailExportRows, so PhaseCounting and PhaseWriting
// agree.
func (s *Store) CountEmailsFiltered(ctx context.Context, f EmailExportFilter) (int, error) {
	q, args := queries.EmailExportCount(filterToExportInput(f))
	var n int
	if err := s.DB.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, errtrace.Wrap(err, "CountEmails")
	}
	return n, nil
}

func filterToExportInput(f EmailExportFilter) queries.EmailExportInput {
	return queries.EmailExportInput{Alias: f.Alias, Since: f.Since, Until: f.Until}
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

// (openedUrlAuditColumns + buildOpenedUrlsQuery moved to internal/store/queries; see P1.9.)

// QueryOpenedUrls streams the OpenedUrls audit table filtered per spec.
// Caller is responsible for `defer rows.Close()`.
func (s *Store) QueryOpenedUrls(ctx context.Context, f OpenedUrlListFilter) (RowsScanner, error) {
	q, args := queries.OpenedUrlsList(queries.OpenedUrlsListInput{
		Before: f.Before, Alias: f.Alias, Origin: f.Origin, Limit: f.Limit,
	})
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, errtrace.Wrap(err, "QueryOpenedUrls")
	}
	return rows, nil
}
