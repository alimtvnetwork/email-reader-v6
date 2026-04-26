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
	"errors"
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

// CountDeletedEmails returns the number of Emails rows with
// `DeletedAt IS NOT NULL` matching the alias (or all when alias ==
// ""). Mirrors the `CountUnreadEmails` shape so
// `(*core.EmailsService).Counts` can issue three independent COUNT
// queries and assemble an `EmailCounts` projection without any
// SUM(CASE) round-trip.
//
// Spec: spec/21-app/02-features/02-emails/01-backend.md §3.5.
func (s *Store) CountDeletedEmails(ctx context.Context, alias string) (int, error) {
	var n int
	var err error
	if alias == "" {
		err = s.DB.QueryRowContext(ctx, queries.EmailsCountDeletedAll).Scan(&n)
	} else {
		err = s.DB.QueryRowContext(ctx, queries.EmailsCountDeletedByAlias, alias).Scan(&n)
	}
	if err != nil {
		return 0, errtrace.Wrap(err, "CountDeletedEmails")
	}
	return n, nil
}

// SetEmailDeletedAt sets `Emails.DeletedAt` for every (alias, uid) in
// `uids`. Returns cumulative RowsAffected across all batches.
//
// **Polarity**: `deletedAt == nil` writes SQL NULL ("undelete"); a
// non-nil pointer writes the dereferenced unix-seconds timestamp
// ("delete with this stamp"). Caller passes a `*int64` (rather than
// two separate methods or a sentinel like `0` meaning NULL) so the
// Delete and Undelete code paths in `*core.EmailsService` share one
// store seam — see `core.EmailsService.Delete`/`Undelete` in
// `internal/core/emails_lifecycle.go`.
//
// **Batching**: same contract as `SetEmailRead` — each UPDATE binds
// at most `queries.SetEmailDeletedAtMaxBatch` (999) UID placeholders;
// larger inputs are chunked inside one transaction so callers see
// all-or-nothing semantics.
//
// **Empty `uids`**: returns (0, nil) without opening a transaction
// — mirrors `SetEmailRead` and keeps `Delete([]uint32{})` a clean
// no-op at every layer.
func (s *Store) SetEmailDeletedAt(ctx context.Context, alias string, uids []uint32, deletedAt *int64) (int64, error) {
	if len(uids) == 0 {
		return 0, nil
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, errtrace.Wrap(err, "SetEmailDeletedAt.Begin")
	}
	var total int64
	for off := 0; off < len(uids); off += queries.SetEmailDeletedAtMaxBatch {
		end := off + queries.SetEmailDeletedAtMaxBatch
		if end > len(uids) {
			end = len(uids)
		}
		q, args := queries.SetEmailDeletedAt(deletedAt, alias, uids[off:end])
		res, err := tx.ExecContext(ctx, q, args...)
		if err != nil {
			_ = tx.Rollback()
			return 0, errtrace.Wrap(err, "SetEmailDeletedAt.Exec")
		}
		n, err := res.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			return 0, errtrace.Wrap(err, "SetEmailDeletedAt.RowsAffected")
		}
		total += n
	}
	if err := tx.Commit(); err != nil {
		return 0, errtrace.Wrap(err, "SetEmailDeletedAt.Commit")
	}
	return total, nil
}

// StoreAccountHealthRow is the projection returned by
// `(*Store).QueryAccountHealth`. Mirrors the **store-derivable**
// subset of `core.AccountHealthRow` — the `Health` and
// `ConsecutiveFailures` fields stay on the core side because:
//
//   - `Health` is computed from the other fields + a clock — pure
//     domain logic, no business in the store.
//   - `ConsecutiveFailures` needs a window-function walk that this
//     slice intentionally defers (see queries.AccountHealthSelectAll
//     doc-comment).
//
// Field name parity with core's `AccountHealthRow` keeps the adapter
// (`core.NewStoreAccountHealthSource`) a five-line copy loop with no
// rename gymnastics. Zero-value timestamps signify "no events yet
// of this kind" — same convention as the core-side row.
type StoreAccountHealthRow struct {
	Alias        string
	LastPollAt   time.Time // zero = never polled (or only Stop/Error events)
	LastErrorAt  time.Time // zero = never errored
	EmailsStored int
	UnreadCount  int
}

// QueryAccountHealth returns one StoreAccountHealthRow per known alias
// — i.e. every alias seen in `WatchEvents` OR `Emails`. Aliases
// configured in `accounts.yaml` but with neither watch events nor
// stored emails do NOT appear; the core service is responsible for
// joining against the configured account list and synthesising
// `Warning` rows for the gap (this matches the existing
// `(*DashboardService).AccountHealth` contract).
//
// **Failure modes**: open errors and Scan errors are wrapped via
// `errtrace.Wrap` with a "QueryAccountHealth" / "QueryAccountHealth.Scan"
// suffix so log readers can tell driver-side failures apart from
// per-row parse failures. Timestamp parse failures are wrapped with
// the offending alias in the message — the most common cause is a
// pre-m0008 hand-imported DB with non-RFC3339 OccurredAt, and naming
// the alias makes it grep-able.
//
// Spec: spec/21-app/01-features/01-dashboard/00-overview.md §4 +
// queries.AccountHealthSelectAll for the SQL rationale.
func (s *Store) QueryAccountHealth(ctx context.Context) ([]StoreAccountHealthRow, error) {
	rows, err := s.DB.QueryContext(ctx, queries.AccountHealthSelectAll)
	if err != nil {
		return nil, errtrace.Wrap(err, "QueryAccountHealth")
	}
	defer rows.Close()

	out := []StoreAccountHealthRow{}
	for rows.Next() {
		var (
			alias              string
			lastPollText       sql.NullString
			lastErrorText      sql.NullString
			emailsStored       int
			unreadCount        int
		)
		if err := rows.Scan(&alias, &lastPollText, &lastErrorText, &emailsStored, &unreadCount); err != nil {
			return nil, errtrace.Wrap(err, "QueryAccountHealth.Scan")
		}
		row := StoreAccountHealthRow{
			Alias:        alias,
			EmailsStored: emailsStored,
			UnreadCount:  unreadCount,
		}
		if lastPollText.Valid && lastPollText.String != "" {
			t, perr := parseSqliteRFC3339(lastPollText.String)
			if perr != nil {
				return nil, errtrace.Wrap(perr, "QueryAccountHealth.LastPollAt["+alias+"]")
			}
			row.LastPollAt = t
		}
		if lastErrorText.Valid && lastErrorText.String != "" {
			t, perr := parseSqliteRFC3339(lastErrorText.String)
			if perr != nil {
				return nil, errtrace.Wrap(perr, "QueryAccountHealth.LastErrorAt["+alias+"]")
			}
			row.LastErrorAt = t
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, errtrace.Wrap(err, "QueryAccountHealth.Rows")
	}
	return out, nil
}

// parseSqliteRFC3339 parses a timestamp in either of the two shapes
// the schema actually emits:
//
//   - `sqliteRFC3339NowExpr` defaulting columns: `2026-04-26T10:05:00.000Z`
//     (RFC3339Nano with millisecond precision and a literal `Z` zone).
//   - Hand-inserted test fixtures: `2026-04-26T10:05:00.000Z` (same
//     shape) or `2026-04-26T10:05:00Z` (no fractional seconds).
//
// We try `time.RFC3339Nano` first (covers both above) then fall back
// to `time.RFC3339` for the no-frac variant. Anything else is a real
// schema drift and surfaces as an error.
func parseSqliteRFC3339(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, errtrace.Wrap(
		errors.New("unrecognised timestamp shape: "+s),
		"parseSqliteRFC3339")
}
