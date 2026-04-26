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
const emailColumns = `Id, Alias, MessageId, Uid, FromAddr, ToAddr, CcAddr, Subject,
       BodyText, BodyHtml, ReceivedAt, FilePath`

// EmailByUid selects a single email row by (Alias, Uid). Static query.
const EmailByUid = `SELECT ` + emailColumns + ` FROM Emails WHERE Alias = ? AND Uid = ?`

// EmailsCountAll counts every row in Emails. Static query.
const EmailsCountAll = `SELECT COUNT(1) FROM Emails`

// EmailsCountByAlias counts rows matching a single alias. Static query.
const EmailsCountByAlias = `SELECT COUNT(1) FROM Emails WHERE Alias = ?`

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
