// Package queries is the single source of truth for SQL strings used by
// the store layer. Keeping every SELECT/INSERT/UPDATE/DELETE statement
// here (and nowhere else under internal/{core,store,exporter,ui,cli})
// lets us:
//
//   - audit the query surface in one place (perf, indices, correctness),
//   - swap dialects without grepping the whole tree,
//   - enforce the boundary with an AST guard test (see P1.9b).
//
// Every exported identifier is either:
//   - a raw SQL string constant (preferred for static queries), or
//   - a small builder function returning (sql, args) for parameterised
//     dynamic shapes (LIKE/limit/offset/sort).
//
// New query? Add it here, add a unit test in queries_test.go, then wire
// the caller to it.
package queries

import (
	"strings"
	"time"
)

// emailColumns is the canonical column list for any SELECT against the
// Emails table. Centralised so a future ALTER TABLE only needs one edit.
// `IsRead` (added by M0010) is included so the Go-side `store.Email`
// struct field of the same name populates on every read path; this in
// turn unblocks the `core.EmailQuery.OnlyUnread` filter (P4.6 follow-up).
const emailColumns = `Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr, Subject,
       BodyText, BodyHtml, ReceivedAt, FilePath, IsRead`

// EmailByUid selects a single email row by (Alias, Uid). Static query.
const EmailByUid = `SELECT ` + emailColumns + ` FROM Emails WHERE Alias = ? AND Uid = ?`

// EmailUpsert inserts a new Emails row; on MessageId conflict it leaves
// the existing row untouched. Caller inspects RowsAffected to learn
// whether the row is new. Static query.
//
// Args bind order: Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
// Subject, BodyText, BodyHtml, ReceivedAt, FilePath.
const EmailUpsert = `INSERT INTO Emails
       (Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr, Subject, BodyText, BodyHtml, ReceivedAt, FilePath)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
       ON CONFLICT(MessageId) DO NOTHING`

// EmailIdByMessageId fetches the existing Emails.Id for a known
// MessageId. Used by UpsertEmail when the INSERT collapsed via the
// ON CONFLICT branch above. Static query.
const EmailIdByMessageId = `SELECT Id FROM Emails WHERE MessageId = ?`

// EmailsCountAll counts every row in Emails. Static query.
const EmailsCountAll = `SELECT COUNT(1) FROM Emails`

// EmailsCountByAlias counts rows matching a single alias. Static query.
const EmailsCountByAlias = `SELECT COUNT(1) FROM Emails WHERE Alias = ?`

// EmailsCountUnreadAll counts every Emails row with `IsRead = 0`.
// Static query. Used by `(*Emails).Counts(alias="")` to render the
// global toolbar/dashboard unread badge.
//
// Spec: `spec/21-app/02-features/02-emails/01-backend.md` §2.6 / §3.5
// (`Counts` method, projecting onto `EmailCounts`). The spec's COUNT
// formula uses a single COALESCE/SUM CASE expression; we emit two
// independent COUNT queries instead so each can hit its own index
// (`IxEmailAliasIsRead` from M0010 covers the Unread variant; the
// existing PK index covers the Total variant). Two round-trips at
// p99 ≪ 5 ms each beats one full-scan SUM.
const EmailsCountUnreadAll = `SELECT COUNT(1) FROM Emails WHERE IsRead = 0`

// EmailsCountUnreadByAlias counts unread rows for one alias. Static
// query — see EmailsCountUnreadAll for rationale.
const EmailsCountUnreadByAlias = `SELECT COUNT(1) FROM Emails WHERE Alias = ? AND IsRead = 0`

// EmailsCountDeletedAll counts every Emails row whose `DeletedAt` is
// non-NULL (Phase 4 P4.3 soft-delete). Static query. Used by
// `(*Emails).Counts(alias="")` to populate `EmailCounts.Deleted`.
//
// **Why `IS NOT NULL` (not `> 0`)** — m0012 chose the
// "NULL == not-deleted" convention specifically so this predicate is
// SARGable against a partial / composite index on `DeletedAt`. A `> 0`
// predicate would also work for our timestamp values but would lock
// us out of `WHERE DeletedAt IS NULL` partial-index recipes if we
// later add one for the inverse hot path. See m0013 for the index.
//
// Spec: `spec/21-app/02-features/02-emails/01-backend.md` §2.6 / §3.5
// (`Counts` method, third COUNT projection onto `EmailCounts.Deleted`).
const EmailsCountDeletedAll = `SELECT COUNT(1) FROM Emails WHERE DeletedAt IS NOT NULL`

// EmailsCountDeletedByAlias counts soft-deleted rows for one alias.
// Static query — see EmailsCountDeletedAll for rationale.
const EmailsCountDeletedByAlias = `SELECT COUNT(1) FROM Emails WHERE Alias = ? AND DeletedAt IS NOT NULL`

// EmailsListInput captures the optional filter + pagination knobs for
// composing the EmailsList query. Mirrors store.EmailQuery — kept as a
// separate type so the queries package has no upward dependency on store.
type EmailsListInput struct {
	Alias  string
	Search string
	Limit  int
	Offset int
}

// EmailsList composes the dynamic ListEmails SQL + bound args. The shape
// is: SELECT <cols> FROM Emails [WHERE ...] ORDER BY Uid DESC, Id DESC
// [LIMIT ? [OFFSET ?]].
func EmailsList(in EmailsListInput) (string, []any) {
	var sb strings.Builder
	sb.WriteString(`SELECT `)
	sb.WriteString(emailColumns)
	sb.WriteString(` FROM Emails`)

	var args []any
	var where []string
	if in.Alias != "" {
		where = append(where, "Alias = ?")
		args = append(args, in.Alias)
	}
	if in.Search != "" {
		where = append(where, "(LOWER(Subject) LIKE ? OR LOWER(FromAddr) LIKE ?)")
		needle := "%" + strings.ToLower(in.Search) + "%"
		args = append(args, needle, needle)
	}
	if len(where) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(where, " AND "))
	}
	sb.WriteString(" ORDER BY Uid DESC, Id DESC")
	if in.Limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, in.Limit)
		if in.Offset > 0 {
			sb.WriteString(" OFFSET ?")
			args = append(args, in.Offset)
		}
	}
	return sb.String(), args
}

// emailExportColumns is the explicit PascalCase column list shared by
// EmailExport and EmailExportCount. No `SELECT *` (AC-DB-D-04 spirit).
const emailExportColumns = `Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr,
                              Subject, BodyText, BodyHtml, ReceivedAt, FilePath, CreatedAt`

// EmailExportInput captures the user-facing filter knobs of ExportCsv.
// Empty Alias means "all aliases"; zero Since/Until means no bound.
// Mirrors store.EmailExportFilter — separate type to keep queries/ free
// of a back-dependency on store.
type EmailExportInput struct {
	Alias string
	Since time.Time
	Until time.Time
}

// EmailExport composes the streaming SELECT for ExportCsv + bound args.
func EmailExport(in EmailExportInput) (string, []any) {
	return buildEmailExportSQL(in, false)
}

// EmailExportCount composes the matching COUNT(*) so PhaseCounting and
// PhaseWriting agree on totals. Same WHERE clause, no ORDER BY.
func EmailExportCount(in EmailExportInput) (string, []any) {
	return buildEmailExportSQL(in, true)
}

func buildEmailExportSQL(in EmailExportInput, count bool) (string, []any) {
	var sb strings.Builder
	if count {
		sb.WriteString(`SELECT COUNT(*) FROM Emails`)
	} else {
		sb.WriteString(`SELECT `)
		sb.WriteString(emailExportColumns)
		sb.WriteString(` FROM Emails`)
	}
	where, args := whereForEmailExport(in)
	if where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(where)
	}
	if !count {
		sb.WriteString(" ORDER BY Id ASC")
	}
	return sb.String(), args
}

func whereForEmailExport(in EmailExportInput) (string, []any) {
	var clauses []string
	var args []any
	if in.Alias != "" {
		clauses = append(clauses, "Alias = ?")
		args = append(args, in.Alias)
	}
	if !in.Since.IsZero() {
		clauses = append(clauses, "ReceivedAt >= ?")
		args = append(args, in.Since.UTC())
	}
	if !in.Until.IsZero() {
		clauses = append(clauses, "ReceivedAt < ?")
		args = append(args, in.Until.UTC())
	}
	return strings.Join(clauses, " AND "), args
}

// ---------------- WatchState ----------------

// WatchStateGet selects the last-seen state row for a single alias.
const WatchStateGet = `SELECT Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt
       FROM WatchState WHERE Alias = ?`

// WatchStateUpsert composes the INSERT…ON CONFLICT for the WatchState
// row owned by one alias. Both the new-row UpdatedAt and the conflict
// branch's UpdatedAt come from the same SQLite RFC3339 "now" expression
// — caller passes that expression in (declared in store/datetime.go) so
// the queries package stays free of dialect drift.
func WatchStateUpsert(nowExpr string) string {
	return `INSERT INTO WatchState (Alias, LastUid, LastSubject, LastReceivedAt, UpdatedAt)
       VALUES (?, ?, ?, ?, ` + nowExpr + `)
       ON CONFLICT(Alias) DO UPDATE SET
           LastUid        = excluded.LastUid,
           LastSubject    = excluded.LastSubject,
           LastReceivedAt = excluded.LastReceivedAt,
           UpdatedAt      = ` + nowExpr
}

// ---------------- OpenedUrls (reads) ----------------

// openedUrlAuditColumns is the explicit Delta-#1 column list.
// Order MUST match core.scanOpenedUrlRows.
const openedUrlAuditColumns = `Id, EmailId, Alias, RuleName, Origin, Url,
                                 OriginalUrl, IsDeduped, IsIncognito, TraceId, OpenedAt`

// HasOpenedUrl checks whether (EmailId, Url) is already recorded.
const HasOpenedUrl = `SELECT COUNT(1) FROM OpenedUrls WHERE EmailId = ? AND Url = ?`

// OpenedUrlInsert is the Delta-#1 rich INSERT for OpenedUrls. On
// (EmailId, Url) conflict the row is left untouched (dedup hit) and
// RowsAffected reports 0. Static query.
//
// Args bind order: EmailId, RuleName, Url, Alias, Origin, OriginalUrl,
// IsDeduped (int 0/1), IsIncognito (int 0/1), TraceId.
const OpenedUrlInsert = `INSERT INTO OpenedUrls (EmailId, RuleName, Url, Alias, Origin,
                               OriginalUrl, IsDeduped, IsIncognito, TraceId)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
       ON CONFLICT(EmailId, Url) DO NOTHING`

// OpenedUrlsListInput captures the filter knobs of RecentOpenedUrls.
// Limit must be > 0; Before is required (zero ⇒ caller passes time.Now()).
type OpenedUrlsListInput struct {
	Before time.Time
	Alias  string
	Origin string
	Limit  int
}

// OpenedUrlsList composes the Delta-#1 audit-row reader SQL + bound args.
func OpenedUrlsList(in OpenedUrlsListInput) (string, []any) {
	var sb strings.Builder
	sb.WriteString(`SELECT `)
	sb.WriteString(openedUrlAuditColumns)
	sb.WriteString(` FROM OpenedUrls WHERE OpenedAt < ?`)
	args := []any{in.Before}
	if in.Alias != "" {
		sb.WriteString(" AND Alias = ?")
		args = append(args, in.Alias)
	}
	if in.Origin != "" {
		sb.WriteString(" AND Origin = ?")
		args = append(args, in.Origin)
	}
	sb.WriteString(" ORDER BY OpenedAt DESC, Id DESC LIMIT ?")
	args = append(args, in.Limit)
	return sb.String(), args
}

// ---------------- Maintenance ----------------

// PruneOpenedUrlsBatched deletes OpenedUrls rows whose OpenedAt is
// strictly older than the bound cutoff, in chunks of LIMIT rows. The
// inner SELECT bounds the DELETE so SQLite can use the OpenedAt index
// without holding the writer lock for an unbounded duration.
//
// Args bind order: cutoff (time), limit (int).
const PruneOpenedUrlsBatched = `DELETE FROM OpenedUrls WHERE rowid IN (
       SELECT rowid FROM OpenedUrls WHERE OpenedAt < ? LIMIT ?
     )`

// ---------------- Email flags (Phase 4 — MarkRead) ----------------

// SetEmailReadMaxBatch caps a single UPDATE statement at the SQLite
// `SQLITE_MAX_VARIABLE_NUMBER` floor (999). Callers that need to mark
// more than 999 UIDs in one logical operation must split into batches
// of this size and wrap them in a single transaction. See spec
// `spec/21-app/02-features/02-emails/01-backend.md` §3.3.
const SetEmailReadMaxBatch = 999

// SetEmailRead composes the UPDATE that flips Emails.IsRead for a
// given (Alias, Uid set). Bind order: read (0/1), alias, uid1, uid2, …
//
// The IN list is rendered with one `?` placeholder per UID (no string
// interpolation of values), so the call site is parameterised end-to-
// end. Caller MUST ensure `len(uids) <= SetEmailReadMaxBatch` and
// `len(uids) > 0` before calling — an empty IN list is invalid SQL.
//
// Spec: M0010 added `IsRead INTEGER NOT NULL DEFAULT 0`. The migration
// note labels the column "boolean-style" (positive form per
// `18-database-conventions.md` §4); we store 1 for read, 0 for unread.
func SetEmailRead(read bool, alias string, uids []uint32) (string, []any) {
	var sb strings.Builder
	sb.WriteString(`UPDATE Emails SET IsRead = ? WHERE Alias = ? AND Uid IN (`)
	for i := range uids {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('?')
	}
	sb.WriteByte(')')

	args := make([]any, 0, 2+len(uids))
	if read {
		args = append(args, 1)
	} else {
		args = append(args, 0)
	}
	args = append(args, alias)
	for _, u := range uids {
		args = append(args, u)
	}
	return sb.String(), args
}

// ---------------- Email lifecycle (Phase 4 — Delete/Undelete, P4.3) ----------------

// SetEmailDeletedAtMaxBatch caps a single UPDATE to the SQLite
// `SQLITE_MAX_VARIABLE_NUMBER` floor (999) — same contract as
// `SetEmailReadMaxBatch`. Larger inputs are split by the store-layer
// shim into batches under one transaction.
const SetEmailDeletedAtMaxBatch = 999

// SetEmailDeletedAt composes the UPDATE that sets `Emails.DeletedAt`
// for a given (Alias, Uid set). Bind order: deletedAtUnix-or-NULL,
// alias, uid1, uid2, …
//
// `deletedAt == nil` writes SQL NULL (the undelete path). Non-nil
// writes the dereferenced int64 (unix-seconds), matching the M0012
// column type.
//
// Caller MUST ensure `len(uids) <= SetEmailDeletedAtMaxBatch` and
// `len(uids) > 0` (an empty IN list is invalid SQL — same precondition
// as `SetEmailRead`).
func SetEmailDeletedAt(deletedAt *int64, alias string, uids []uint32) (string, []any) {
	var sb strings.Builder
	sb.WriteString(`UPDATE Emails SET DeletedAt = ? WHERE Alias = ? AND Uid IN (`)
	for i := range uids {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('?')
	}
	sb.WriteByte(')')

	args := make([]any, 0, 2+len(uids))
	if deletedAt == nil {
		args = append(args, nil) // SQL NULL = "not deleted"
	} else {
		args = append(args, *deletedAt)
	}
	args = append(args, alias)
	for _, u := range uids {
		args = append(args, u)
	}
	return sb.String(), args
}

// AccountHealthSelectAll returns one row per known alias (union of
// every alias seen in WatchEvents OR Emails) with the four
// store-derived columns the Dashboard `AccountHealth` projection
// needs:
//
//   - LastPollAt   — most recent OccurredAt where Kind IN (1, 4)
//                    i.e. WatchEventStart (1) or WatchEventHeartbeat (4).
//                    Both signal "the watcher is alive at this instant",
//                    so MAX over that pair is the cleanest "last poll"
//                    proxy without introducing a new event kind.
//   - LastErrorAt  — most recent OccurredAt where Kind = 3 (Error).
//   - EmailsStored — COUNT(1) over Emails grouped by Alias.
//   - UnreadCount  — SUM(IsRead = 0) over Emails grouped by Alias.
//
// **What this intentionally does NOT compute:**
//
//   - `ConsecutiveFailures` — requires a window-function walk
//     (count trailing Kind=3 events back to the most recent non-3
//     event). SQLite's `lag()`/`row_number()` would work but adds a
//     CTE per alias and the field is only consulted by the Health
//     "≥3" branch in `core.ComputeHealth`. Slice #102 ships the
//     four cheap projections; ConsecutiveFailures stays at zero
//     until a follow-on slice teaches the watcher to write a
//     dedicated counter column to WatchState (the natural home —
//     no SQL gymnastics, single UPDATE per poll outcome).
//   - `Health` — purely derived in `core.ComputeHealth` from the
//     other fields + clock; the store has no business computing it.
//
// **Why the LEFT JOIN structure (not a single GROUP BY)** — the
// natural alias set is `WatchEvents ∪ Emails` (an account can have
// no watch events yet but a back-fill of emails, or vice versa). A
// single GROUP BY on either table would silently drop the other
// half. The CTE union materialises the alias set explicitly; the
// three left joins each contribute their slice of columns; missing
// rows COALESCE to zero / SQL NULL (which scans as a zero
// `time.Time` on the Go side via `sql.NullString`).
//
// **Why timestamps as TEXT (not unix-seconds)** — `WatchEvents.OccurredAt`
// is RFC3339 TEXT (m0008's canonical timestamp convention for that
// table). Caller parses on the Go side via `time.Parse(time.RFC3339Nano, …)`.
//
// Spec: spec/21-app/01-features/01-dashboard/00-overview.md §4 +
// roadmap-phases.md §PHASE 3 (deferred Store.QueryAccountHealth shim).
const AccountHealthSelectAll = `
WITH
  alias_set AS (
    SELECT DISTINCT Alias FROM WatchEvents
    UNION
    SELECT DISTINCT Alias FROM Emails
  ),
  poll AS (
    SELECT Alias, MAX(OccurredAt) AS LastPollAt
    FROM WatchEvents WHERE Kind IN (1, 4) GROUP BY Alias
  ),
  err AS (
    SELECT Alias, MAX(OccurredAt) AS LastErrorAt
    FROM WatchEvents WHERE Kind = 3 GROUP BY Alias
  ),
  emails_agg AS (
    SELECT Alias,
           COUNT(1) AS Stored,
           SUM(CASE WHEN IsRead = 0 THEN 1 ELSE 0 END) AS Unread
    FROM Emails GROUP BY Alias
  )
SELECT a.Alias,
       p.LastPollAt,
       e.LastErrorAt,
       COALESCE(em.Stored, 0) AS EmailsStored,
       COALESCE(em.Unread, 0) AS UnreadCount
FROM alias_set a
LEFT JOIN poll       p  ON p.Alias  = a.Alias
LEFT JOIN err        e  ON e.Alias  = a.Alias
LEFT JOIN emails_agg em ON em.Alias = a.Alias
ORDER BY a.Alias`
