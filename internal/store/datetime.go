// datetime.go centralises the datetime storage format for the
// `internal/store/...` package. All `*At` columns are persisted as
// RFC 3339 UTC strings with millisecond precision and a literal `Z`
// suffix — the format mandated by spec/23-app-database/01-schema.md X-4
// and asserted by AC-DB-53 (`Test_DateTime_FormatUtc`).
//
// Two surfaces use this helper:
//
//   1. Go-side parameter binding (`UpsertEmail`, `UpsertWatchState`,
//      `RecordOpenedUrlExt`, …): bind `formatRFC3339UTC(t)` instead of
//      raw `time.Time`, so modernc/sqlite can't fall back to its
//      default `"2006-01-02 15:04:05.999999999 -0700 MST"` stringer.
//   2. SQLite-side defaults (the `DEFAULT (...)` expressions in
//      `migrate()`): use `sqliteRFC3339NowExpr` so `CURRENT_TIMESTAMP`
//      doesn't sneak the legacy `"2006-01-02 15:04:05"` form into
//      auto-populated `CreatedAt` / `UpdatedAt` / `OpenedAt` columns.
//
// Reads are unchanged: modernc/sqlite's `sql.NullTime` parser accepts
// RFC 3339 UTC just as readily as the legacy space-separated form.
package store

import "time"

// rfc3339UTCMillis is the canonical wire format. Millisecond precision
// matches the X-4 spec example `strftime('%Y-%m-%dT%H:%M:%fZ','now')`,
// where SQLite's `%f` produces seconds-with-fractional-millis.
const rfc3339UTCMillis = "2006-01-02T15:04:05.000Z"

// sqliteRFC3339NowExpr is the SQL fragment to substitute for
// `CURRENT_TIMESTAMP` in `DEFAULT (...)` clauses and in INSERT/UPDATE
// statements that need a server-side "now" value. Wrapped in parens
// because SQLite requires non-literal defaults to be parenthesised.
const sqliteRFC3339NowExpr = `(strftime('%Y-%m-%dT%H:%M:%fZ','now'))`

// formatRFC3339UTC renders t as `YYYY-MM-DDTHH:MM:SS.sssZ`. A zero
// `time.Time` returns "" so callers can pass the result straight into
// a nullable column binding (modernc/sqlite treats "" as NULL for
// DATETIME columns when the column is declared NULLable).
func formatRFC3339UTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(rfc3339UTCMillis)
}
