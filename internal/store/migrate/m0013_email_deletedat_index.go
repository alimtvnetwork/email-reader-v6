// m0013_email_deletedat_index.go adds a composite index on the
// soft-delete column m0012 introduced:
//
//	CREATE INDEX IF NOT EXISTS IxEmailsAliasDeletedAt
//	    ON Emails(Alias, DeletedAt)
//
// **Why composite (Alias, DeletedAt) and not a partial index** —
// Slice #100's `EmailsCountDeletedByAlias` filters on both columns
// (`WHERE Alias = ? AND DeletedAt IS NOT NULL`), so a leading-Alias
// composite serves both the per-alias COUNT and the global-aggregate
// COUNT (`SELECT COUNT(1) ... WHERE DeletedAt IS NOT NULL` can scan
// the index leaves directly). A partial index `WHERE DeletedAt IS
// NOT NULL` would shrink storage further but would not help the
// future "list non-deleted rows" hot path that `emails_query.go`
// promises in P4.6 — keeping the inclusive composite leaves both
// directions open without the caller juggling two indexes.
//
// **Why raw `Up` (not `UpFunc`)** — `CREATE INDEX IF NOT EXISTS` is
// natively idempotent in SQLite, unlike `ALTER TABLE ADD COLUMN`
// (m0010 / m0012's reason for needing PRAGMA gating).
package migrate

func init() {
	Register(Migration{
		Version: 13,
		Name:    "email_deletedat_index",
		Up:      `CREATE INDEX IF NOT EXISTS IxEmailsAliasDeletedAt ON Emails(Alias, DeletedAt)`,
	})
}
