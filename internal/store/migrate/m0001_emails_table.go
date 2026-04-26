// m0001_emails_table.go creates the canonical Emails table.
//
// Pre-P1.11, this DDL lived inline in `Store.migrate()` (the first
// statement of the `stmts` slice). Behaviour is unchanged: same column
// types, same defaults, same `MessageId UNIQUE` constraint that drives
// `UpsertEmail`'s ON CONFLICT branch.
//
// Idempotency: `CREATE TABLE IF NOT EXISTS` makes this safe on
// upgraded DBs (where the table predates `_SchemaVersion`). On those
// DBs the harness re-runs this Up on first post-upgrade boot; the
// statement is a no-op and the ledger row is recorded.
package migrate

func init() {
	Register(Migration{
		Version: 1,
		Name:    "emails_table",
		Up: `CREATE TABLE IF NOT EXISTS Emails (
                Id          INTEGER PRIMARY KEY AUTOINCREMENT,
                Alias       TEXT    NOT NULL,
                MessageId   TEXT    NOT NULL UNIQUE,
                Uid         INTEGER NOT NULL,
                FromAddr    TEXT    NOT NULL DEFAULT '',
                ToAddr      TEXT    NOT NULL DEFAULT '',
                CcAddr      TEXT    NOT NULL DEFAULT '',
                Subject     TEXT    NOT NULL DEFAULT '',
                BodyText    TEXT    NOT NULL DEFAULT '',
                BodyHtml    TEXT    NOT NULL DEFAULT '',
                ReceivedAt  DATETIME,
                FilePath    TEXT    NOT NULL DEFAULT '',
                CreatedAt   DATETIME NOT NULL DEFAULT ` + sqliteRFC3339NowExpr + `
              )`,
	})
}
