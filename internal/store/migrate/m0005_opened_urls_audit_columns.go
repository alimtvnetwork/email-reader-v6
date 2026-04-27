// m0005_opened_urls_audit_columns.go adds the six Delta-#1 audit
// columns to the OpenedUrls table. SQLite's `ALTER TABLE ADD COLUMN`
// has no `IF NOT EXISTS` form and errors with "duplicate column name"
// on re-execution, so we use the `UpFunc` form to introspect
// `PRAGMA table_info(OpenedUrls)` and skip ADDs that already landed.
//
// This is the canonical PRAGMA-gated migration pattern for the
// codebase. New ALTER-style migrations should follow this shape.
//
// Spec: spec/21-app/02-features/06-tools/01-backend.md §2.5 + Delta #1.
package migrate

import (
	"context"
	"database/sql"

	"github.com/lovable/email-read/internal/errtrace"
)

func init() {
	Register(Migration{
		Version: 5,
		Name:    "opened_urls_audit_columns",
		UpFunc:  applyOpenedUrlsAuditColumns,
	})
}

// applyOpenedUrlsAuditColumns is the introspection-gated body for
// m0005. Mirrors the pre-P1.11 logic of `Store.migrateOpenedUrlsDelta1`.
func applyOpenedUrlsAuditColumns(ctx context.Context, db *sql.DB) error {
	have, err := openedUrlsColumns(ctx, db)
	if err != nil {
		return errtrace.Wrapf(err, "introspect OpenedUrls")
	}
	adds := []struct{ name, ddl string }{
		{"Alias", `ALTER TABLE OpenedUrls ADD COLUMN Alias TEXT NOT NULL DEFAULT ''`},
		{"Origin", `ALTER TABLE OpenedUrls ADD COLUMN Origin TEXT NOT NULL DEFAULT ''`},
		{"OriginalUrl", `ALTER TABLE OpenedUrls ADD COLUMN OriginalUrl TEXT NOT NULL DEFAULT ''`},
		{"IsDeduped", `ALTER TABLE OpenedUrls ADD COLUMN IsDeduped INTEGER NOT NULL DEFAULT 0`},
		{"IsIncognito", `ALTER TABLE OpenedUrls ADD COLUMN IsIncognito INTEGER NOT NULL DEFAULT 0`},
		{"TraceId", `ALTER TABLE OpenedUrls ADD COLUMN TraceId TEXT NOT NULL DEFAULT ''`},
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

// openedUrlsColumns returns the set of column names currently on the
// OpenedUrls table. Local copy of the same helper that previously
// lived in `internal/store/store.go`; kept private to the migrate
// package so future migrations can reuse it.
func openedUrlsColumns(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(OpenedUrls)`)
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
