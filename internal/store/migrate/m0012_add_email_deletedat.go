// m0012_add_email_deletedat.go adds the soft-delete timestamp column
// to the Emails table:
//
//   - `DeletedAt` — INTEGER NULL (unix-seconds, NULL = not deleted).
//
// **Why INTEGER NULL (not TEXT ISO-8601, not "IsDeleted INTEGER")**
//
//   - INTEGER unix-seconds matches every other timestamp column in the
//     schema (`OpenedUrls.OpenedAt`, `WatchEvents.At`, etc.) — the
//     codebase has standardised on `int64` storage with Go-side
//     `time.Time` projection, avoiding the date-string parse cost on
//     every query.
//   - NULL-vs-set is the soft-delete signal. A separate `IsDeleted`
//     boolean would force every undelete to issue two writes (clear
//     the flag, clear the timestamp) and risks the two columns drifting
//     out of sync. One column = one source of truth.
//   - Spec hook: `core.EmailsService.Delete` will write `DeletedAt =
//     clock().Unix()` (P4.3, this slice's caller); `Undelete` writes
//     `DeletedAt = NULL`. The query layer's `IncludeDeleted` flag
//     (currently a documented no-op in `emails_query.go`) becomes
//     functional in the next slice when `EmailsList` learns the
//     `WHERE DeletedAt IS NULL` filter.
//
// **Why UpFunc + PRAGMA (not plain DDL)** — same rationale as m0010
// (`add_email_flags`): SQLite's `ALTER TABLE ADD COLUMN` has no `IF
// NOT EXISTS` form. The PRAGMA-introspection gate makes the migration
// safe to re-run on hand-imported user DBs that pre-date the
// `_SchemaVersion` ledger.
package migrate

import (
	"context"
	"database/sql"

	"github.com/lovable/email-read/internal/errtrace"
)

func init() {
	Register(Migration{
		Version: 12,
		Name:    "add_email_deletedat",
		UpFunc:  applyAddEmailDeletedAt,
	})
}

// applyAddEmailDeletedAt adds the `DeletedAt` column iff it is not
// already present. Reuses `emailsColumns` from m0010 (same package)
// for the introspection — keeps the PRAGMA-walk in one place.
func applyAddEmailDeletedAt(ctx context.Context, db *sql.DB) error {
	have, err := emailsColumns(ctx, db)
	if err != nil {
		return errtrace.Wrapf(err, "introspect Emails")
	}
	if have["DeletedAt"] {
		return nil
	}
	// NULL default (no `DEFAULT` clause needed — SQLite columns are
	// nullable by default unless NOT NULL is specified). Pre-existing
	// rows pick up NULL, which is exactly the "not deleted" sentinel
	// — no backfill required.
	if _, err := db.ExecContext(ctx, `ALTER TABLE Emails ADD COLUMN DeletedAt INTEGER`); err != nil {
		return errtrace.Wrapf(err, "add column DeletedAt")
	}
	return nil
}
