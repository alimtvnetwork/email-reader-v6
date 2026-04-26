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
