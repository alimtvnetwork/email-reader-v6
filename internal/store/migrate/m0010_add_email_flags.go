// m0010_add_email_flags.go adds two boolean-style columns to the
// Emails table:
//
//   - `IsRead`    — 1 once the user has opened/viewed the email in
//                   the UI (or marked-as-read via CLI). Default 0.
//   - `IsFlagged` — 1 when the user has starred/pinned the email
//                   (independent of IsRead). Default 0.
//
// Both columns are stored as INTEGER (0/1) — SQLite has no native
// BOOLEAN; INTEGER matches the pattern already used for
// `OpenedUrls.IsDeduped` / `IsIncognito` (Delta-#1). Defaulting to 0
// keeps every pre-existing row in a well-defined state without
// requiring a backfill UPDATE.
//
// **Why UpFunc + PRAGMA, not plain DDL?** SQLite's `ALTER TABLE ADD
// COLUMN` has no `IF NOT EXISTS` form. On user DBs that have already
// been re-opened once after this migration's ledger row was recorded,
// re-running the raw `ALTER` would error with "duplicate column
// name". The harness's ledger short-circuit normally prevents that,
// but we follow the same defensive PRAGMA-gated pattern as m0005
// (`opened_urls_audit_columns`) so:
//
//   - test fixtures that bypass `_SchemaVersion` (rare, but possible)
//     still work,
//   - hand-imported user DBs that pre-date the harness still work,
//   - the migration is self-documenting as the canonical idempotent
//     `ALTER TABLE ADD COLUMN` recipe.
//
// Spec hook: future Emails view (Phase 2 Emails service) will read
// these columns; the Watch service may auto-set `IsRead=1` when
// rules.OpenUrl successfully launches the URL.
package migrate

import (
	"context"
	"database/sql"

	"github.com/lovable/email-read/internal/errtrace"
)

func init() {
	Register(Migration{
		Version: 10,
		Name:    "add_email_flags",
		UpFunc:  applyAddEmailFlags,
	})
}

// applyAddEmailFlags introspects `PRAGMA table_info(Emails)` and only
// emits ADDs for columns that are currently missing.
func applyAddEmailFlags(ctx context.Context, db *sql.DB) error {
	have, err := emailsColumns(ctx, db)
	if err != nil {
		return errtrace.Wrapf(err, "introspect Emails")
	}
	adds := []struct{ name, ddl string }{
		{"IsRead", `ALTER TABLE Emails ADD COLUMN IsRead INTEGER NOT NULL DEFAULT 0`},
		{"IsFlagged", `ALTER TABLE Emails ADD COLUMN IsFlagged INTEGER NOT NULL DEFAULT 0`},
	}
	for _, a := range adds {
		if have[a.name] {
			continue
		}
		if _, err := db.ExecContext(ctx, a.ddl); err != nil {
			return errtrace.Wrapf(err, "add column %s", a.name)
		}
	}
	return nil
}

// emailsColumns returns the set of column names currently present on
// the Emails table. Mirrors `openedUrlsColumns` (m0005) but for
// Emails — kept as a separate function (vs. a generic
// `tableColumns(table string)`) because injecting a table name into
// `PRAGMA table_info(?)` requires string interpolation, which the
// codebase intentionally avoids in the SQL layer.
func emailsColumns(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(Emails)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var (
			cid       int
			name, typ string
			notnull   int
			dflt      sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}
