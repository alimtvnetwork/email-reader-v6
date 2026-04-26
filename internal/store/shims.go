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

// SetEmailRead flips Emails.IsRead for every (alias, uid) pair in
// `uids`. Returns the cumulative RowsAffected across all batches.
//
// **Batching contract** (per spec/21-app/02-features/02-emails/01-backend.md
// §3.3): a single UPDATE may bind at most
// `queries.SetEmailReadMaxBatch` (999) UID placeholders — the SQLite
// `SQLITE_MAX_VARIABLE_NUMBER` floor. We split larger inputs into
// chunks of that size and wrap the whole sequence in one
// transaction so callers see all-or-nothing semantics: any chunk
// failing rolls back every preceding chunk in the same call.
//
// **Empty `uids`**: returns (0, nil) without opening a transaction
// (matches the spec's "empty UIDs → no SQL" branch and keeps the
// `MarkRead_EmptyUids_NoSql` test deterministic).
//
// **Idempotency**: SQLite's UPDATE returns RowsAffected even when the
// new value equals the old — so re-issuing the same op may report
// nonzero changes. The spec's idempotency contract is *behavioral*
// (final state matches), not *RowsAffected = 0 on repeat*; tests
// assert the former.
func (s *Store) SetEmailRead(ctx context.Context, alias string, uids []uint32, read bool) (int64, error) {
	if len(uids) == 0 {
		return 0, nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, errtrace.Wrap(err, "SetEmailRead.Begin")
	}
	var total int64
	for off := 0; off < len(uids); off += queries.SetEmailReadMaxBatch {
		end := off + queries.SetEmailReadMaxBatch
		if end > len(uids) {
			end = len(uids)
		}
		q, args := queries.SetEmailRead(read, alias, uids[off:end])
		res, err := tx.ExecContext(ctx, q, args...)
		if err != nil {
			_ = tx.Rollback()
			return 0, errtrace.Wrap(err, "SetEmailRead.Exec")
		}
		n, err := res.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			return 0, errtrace.Wrap(err, "SetEmailRead.RowsAffected")
		}
		total += n
	}
	if err := tx.Commit(); err != nil {
		return 0, errtrace.Wrap(err, "SetEmailRead.Commit")
	}
	return total, nil
}

// CountUnreadEmails returns the number of Emails rows with `IsRead =
// 0` matching the alias (or all when alias == ""). Mirrors the
// `CountEmails` shape so `(*core.Emails).Counts` can issue two
// independent COUNT queries and assemble an `EmailCounts` projection
// without any SUM(CASE) round-trip.
//
// Spec: spec/21-app/02-features/02-emails/01-backend.md §3.5.
func (s *Store) CountUnreadEmails(ctx context.Context, alias string) (int, error) {
	var n int
	var err error
	if alias == "" {
		err = s.DB.QueryRowContext(ctx, queries.EmailsCountUnreadAll).Scan(&n)
	} else {
		err = s.DB.QueryRowContext(ctx, queries.EmailsCountUnreadByAlias, alias).Scan(&n)
	}
	if err != nil {
		return 0, errtrace.Wrap(err, "CountUnreadEmails")
	}
	return n, nil
}

